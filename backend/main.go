package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
)

// ---------------------------------------------------------
// VERİ YAPILARI
// ---------------------------------------------------------

// Tenant: Sistemdeki bir müşteriyi temsil eder.
type Tenant struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Plan  string `json:"plan"`
}

// LogEntry: Müşteri sunucusundan gelen ham log satırı.
type LogEntry struct {
	TenantID  string    `json:"tenant_id"`
	Source    string    `json:"source"`   // sshd, postgresql, firewall, custom
	RawLog    string    `json:"raw_log"`
	Timestamp time.Time `json:"timestamp"`
}

// Alert: Pattern Engine tarafından üretilen güvenlik uyarısı.
type Alert struct {
	ID        int64     `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Type      string    `json:"type"`     // brute_force, unauthorized_access, port_scan
	Severity  string    `json:"severity"` // critical, high, medium, low
	SourceIP  string    `json:"source_ip"`
	Username  string    `json:"username"`
	Message   string    `json:"message"`
	RawLog    string    `json:"raw_log"`
	Timestamp time.Time `json:"timestamp"`
}

// IngestRequest: /ingest endpoint'ine gelen istek gövdesi.
type IngestRequest struct {
	Source string `json:"source" binding:"required"`
	RawLog string `json:"raw_log" binding:"required"`
}

// ---------------------------------------------------------
// GLOBAL DEĞİŞKENLER
// ---------------------------------------------------------

var (
	db          *sql.DB
	rdb         *redis.Client
	logQueue    = make(chan LogEntry, 500)
	alertQueue  = make(chan Alert, 100)
	flowQueue   = make(chan FlowEvent, 1000)

	// Tenant başına WebSocket bağlantıları
	clients   = make(map[string][]*websocket.Conn)
	clientsMu sync.Mutex

	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	// Brute-force sayacı: "tenantID:ip" -> deneme sayısı + zaman
	bfCounters   = make(map[string]*BruteForceCounter)
	bfCountersMu sync.Mutex

	alertIDCounter int64

	// Dinamik Topoloji Verileri
	currentTopology = TopologyData{Nodes: []TopologyNode{}, Links: []TopologyLink{}}
	topologyMu      sync.RWMutex

	// JARVIS — Engelli IP'ler ve son alertler (bellek)
	blockedIPs      = make(map[string]bool)
	blockedIPsMu    sync.RWMutex
	recentAlertsMem []Alert
	recentAlertsMu  sync.RWMutex
)

// BruteForceCounter: Belirli bir IP için başarısız giriş sayacı
type BruteForceCounter struct {
	Count     int
	FirstSeen time.Time
}

type TopologyNode struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Val         int    `json:"val"`
	Group       int    `json:"group"`
	Color       string `json:"color"`
	Parent      string `json:"parent,omitempty"`
	Description string `json:"description,omitempty"`
	Tech        string `json:"tech,omitempty"`
	NodeType    string `json:"node_type,omitempty"`
}

type TopologyLink struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Val    int    `json:"val"`
	Label  string `json:"label,omitempty"`
}

type TopologyData struct {
	Nodes []TopologyNode `json:"nodes"`
	Links []TopologyLink `json:"links"`
}

// UptimeStatus: Faz 4.5 Uptime monitöring verisi
type UptimeStatus struct {
	Service string  `json:"service"`
	Status  string  `json:"status"` // up, down, degraded
	Latency int     `json:"latency"`
	Uptime  float64 `json:"uptime_percent"`
}

var (
	uptimeData         = make(map[string]*UptimeStatus)
	uptimeDataMu       sync.RWMutex
	kafkaStreamEnabled = false
	kafkaMu            sync.Mutex
	kafkaWriter        *kafka.Writer
	kafkaAddr          = "kafka:9092"
)

// FlowEvent: Gerçek zamanlı veri akışı olayı
type FlowEvent struct {
	MsgType  string `json:"msg_type"` // "flow"
	TenantID string `json:"-"`
	Source   string `json:"source"`
	Target   string `json:"target"`
}

// mapLogToFlows: Log kaynağını topoloji bağlantılarına eşler.
// Artık sadece log kaynağı → backend arası gerçek akışı yansıtır.
func mapLogToFlows(entry LogEntry) []FlowEvent {
	// Mevcut topoloji bağlantılarından gerçek flow üret
	topologyMu.RLock()
	links := currentTopology.Links
	topologyMu.RUnlock()

	events := make([]FlowEvent, 0)
	for _, link := range links {
		src := link.Source
		dst := link.Target
		if src == entry.Source || dst == entry.Source {
			events = append(events, FlowEvent{
				MsgType:  "flow",
				TenantID: entry.TenantID,
				Source:   src,
				Target:   dst,
			})
		}
	}
	return events
}

// ---------------------------------------------------------
// DINAMİK TOPOLOJİ KEŞFİ (Auto-Discovery from Data Flows)
// ---------------------------------------------------------
func registerDiscoveredFlow(src, dst string) {
	topologyMu.Lock()
	defer topologyMu.Unlock()

	nodeExists := func(id string) bool {
		for _, n := range currentTopology.Nodes {
			if n.ID == id {
				return true
			}
		}
		return false
	}

	linkExists := func(s, t string) bool {
		for _, l := range currentTopology.Links {
			if l.Source == s && l.Target == t {
				return true
			}
		}
		return false
	}

	if !nodeExists(src) {
		color := "#3b82f6"
		if strings.Contains(src, "db") { color = "#8b5cf6" } else if strings.Contains(src, "redis") { color = "#ef4444" }
		currentTopology.Nodes = append(currentTopology.Nodes, TopologyNode{
			ID: src, Name: src, Val: 6, Group: 4, Color: color, NodeType: "auto-discovered",
		})
	}
	if !nodeExists(dst) {
		color := "#3b82f6"
		if strings.Contains(dst, "db") { color = "#8b5cf6" } else if strings.Contains(dst, "redis") { color = "#ef4444" }
		currentTopology.Nodes = append(currentTopology.Nodes, TopologyNode{
			ID: dst, Name: dst, Val: 6, Group: 4, Color: color, NodeType: "auto-discovered",
		})
	}
	if !linkExists(src, dst) {
		currentTopology.Links = append(currentTopology.Links, TopologyLink{
			Source: src, Target: dst, Val: 3, Label: "live-flow",
		})
	}
}

// ---------------------------------------------------------
// PATTERN ENGINE — REGEX KURALLARI
// ---------------------------------------------------------

var (
	// SSH başarısız giriş: "Failed password for root from 192.168.1.1 port 22"
	reSSHFailed = regexp.MustCompile(
		`(?i)Failed password for (?:invalid user )?(\S+) from (\d+\.\d+\.\d+\.\d+)`,
	)
	// Geçersiz kullanıcı: "Invalid user admin from 192.168.1.1"
	reInvalidUser = regexp.MustCompile(
		`(?i)Invalid user (\S+) from (\d+\.\d+\.\d+\.\d+)`,
	)
	// PostgreSQL kimlik doğrulama hatası
	reDBFailed = regexp.MustCompile(
		`(?i)FATAL:\s+password authentication failed for user "?(\S+?)"?`,
	)
	// Sudo yetkisiz erişim denemesi
	reSudoFailed = regexp.MustCompile(
		`(?i)sudo:.*user (\S+).*NOT in sudoers`,
	)
	// SQL Injection Denemesi (Temel pattern'lar)
	reSQLInjection = regexp.MustCompile(
		`(?i)(UNION\s+SELECT|1\s*=\s*1|--|OR\s+1=1|\bDROP\s+TABLE\b)`,
	)
	// Firewall Drop (Port Scan şüphesi)
	reFirewallDrop = regexp.MustCompile(
		`(?i)DROP IN=\S+ SRC=(\d+\.\d+\.\d+\.\d+)`,
	)
	// IP adresi çıkarma (genel)
	reIP = regexp.MustCompile(`\d+\.\d+\.\d+\.\d+`)
)

// ---------------------------------------------------------
// PATTERN ENGINE — Analiz Fonksiyonu
// ---------------------------------------------------------

// analyzeLog: Gelen log satırını analiz eder, tehdit varsa Alert üretir.
func analyzeLog(entry LogEntry) *Alert {
	raw := entry.RawLog

	// --- Kural 1: SSH Başarısız Giriş ---
	if matches := reSSHFailed.FindStringSubmatch(raw); len(matches) == 3 {
		username := matches[1]
		ip := matches[2]

		// Brute-force sayacını güncelle
		severity := updateBruteForceCounter(entry.TenantID, ip)
		if severity == "" {
			return nil // Henüz eşiğe ulaşmadı
		}

		return &Alert{
			TenantID: entry.TenantID,
			Type:     "brute_force",
			Severity: severity,
			SourceIP: ip,
			Username: username,
			Message:  fmt.Sprintf("SSH brute-force saldırısı: %s kullanıcısı, IP: %s", username, ip),
			RawLog:   raw,
		}
	}

	// --- Kural 2: Geçersiz Kullanıcı Girişimi ---
	if matches := reInvalidUser.FindStringSubmatch(raw); len(matches) == 3 {
		username := matches[1]
		ip := matches[2]
		return &Alert{
			TenantID: entry.TenantID,
			Type:     "unauthorized_access",
			Severity: "medium",
			SourceIP: ip,
			Username: username,
			Message:  fmt.Sprintf("Yetkisiz erişim denemesi: Bilinmeyen kullanıcı '%s', IP: %s", username, ip),
			RawLog:   raw,
		}
	}

	// --- Kural 3: Veritabanı Kimlik Doğrulama Hatası ---
	if matches := reDBFailed.FindStringSubmatch(raw); len(matches) == 2 {
		username := matches[1]
		ip := reIP.FindString(raw)
		return &Alert{
			TenantID: entry.TenantID,
			Type:     "db_breach_attempt",
			Severity: "high",
			SourceIP: ip,
			Username: username,
			Message:  fmt.Sprintf("Veritabanı yetkisiz erişim: '%s' kullanıcısı", username),
			RawLog:   raw,
		}
	}

	// --- Kural 4: Sudoers İhlali ---
	if matches := reSudoFailed.FindStringSubmatch(raw); len(matches) == 2 {
		username := matches[1]
		return &Alert{
			TenantID: entry.TenantID,
			Type:     "privilege_escalation",
			Severity: "high",
			SourceIP: "",
			Username: username,
			Message:  fmt.Sprintf("Yetki yükseltme denemesi: '%s' kullanıcısı sudoers'da değil", username),
			RawLog:   raw,
		}
	}

	// --- Kural 5: SQL Injection Denemesi ---
	if reSQLInjection.MatchString(raw) {
		ip := reIP.FindString(raw)
		return &Alert{
			TenantID: entry.TenantID,
			Type:     "sqli_attempt",
			Severity: "critical",
			SourceIP: ip,
			Username: "",
			Message:  "Olası SQL Injection saldırısı tespit edildi",
			RawLog:   raw,
		}
	}

	// --- Kural 6: Firewall Drop (Olası Port Tarama) ---
	if matches := reFirewallDrop.FindStringSubmatch(raw); len(matches) == 2 {
		ip := matches[1]
		return &Alert{
			TenantID: entry.TenantID,
			Type:     "port_scan",
			Severity: "low",
			SourceIP: ip,
			Username: "",
			Message:  fmt.Sprintf("Şüpheli trafik engellendi (Firewall Drop), IP: %s", ip),
			RawLog:   raw,
		}
	}

	return nil // Tehdit tespit edilmedi
}

// updateBruteForceCounter: IP bazlı sayacı günceller.
// 60 saniyede 5+ deneme = high, 10+ deneme = critical
func updateBruteForceCounter(tenantID, ip string) string {
	key := tenantID + ":" + ip

	bfCountersMu.Lock()
	defer bfCountersMu.Unlock()

	counter, exists := bfCounters[key]
	if !exists || time.Since(counter.FirstSeen) > 60*time.Second {
		bfCounters[key] = &BruteForceCounter{Count: 1, FirstSeen: time.Now()}
		return ""
	}

	counter.Count++
	if counter.Count >= 10 {
		return "critical"
	}
	if counter.Count >= 5 {
		return "high"
	}
	return "" // Henüz eşiğe ulaşmadı
}

// ---------------------------------------------------------
// WORKER — Log İşleme Goroutine'i
// ---------------------------------------------------------

func worker(id int, wg *sync.WaitGroup) {
	defer wg.Done()
	for entry := range logQueue {
		// Topoloji akış olaylarını gönder (non-blocking)
		for _, flow := range mapLogToFlows(entry) {
			// Dinamik topolojiye ekle
			registerDiscoveredFlow(flow.Source, flow.Target)
			
			select {
			case flowQueue <- flow:
			default:
			}
		}

		// Pattern Engine'e gönder
		alert := analyzeLog(entry)
		if alert == nil {
			saveLogToDB(entry)
			continue
		}

		// Tehdit tespit edildi!
		alert.ID = atomic.AddInt64(&alertIDCounter, 1)
		alert.Timestamp = time.Now()

		fmt.Printf("🚨 Alert [%s]: %s\n", alert.Severity, alert.Message)

		saveLogToDB(entry)
		saveAlertToDB(*alert)

		// Bellekte son 50 alert'i sakla (JARVIS için)
		recentAlertsMu.Lock()
		recentAlertsMem = append([]Alert{*alert}, recentAlertsMem...)
		if len(recentAlertsMem) > 50 {
			recentAlertsMem = recentAlertsMem[:50]
		}
		recentAlertsMu.Unlock()

		alertQueue <- *alert
	}
}

// ---------------------------------------------------------
// DATABASE YARDIMCI FONKSİYONLARI
// ---------------------------------------------------------

func saveLogToDB(entry LogEntry) {
	if db == nil {
		return
	}
	ip := reIP.FindString(entry.RawLog)
	_, err := db.Exec(
		`INSERT INTO log_entries (tenant_id, source, raw_log, source_ip, created_at)
		 VALUES ($1, $2, $3, NULLIF($4,'')::inet, $5)`,
		entry.TenantID, entry.Source, entry.RawLog, ip, entry.Timestamp,
	)
	if err != nil {
		fmt.Println("Log DB hatası:", err)
	}
}

func saveAlertToDB(alert Alert) {
	if db == nil {
		return
	}
	_, err := db.Exec(
		`INSERT INTO alerts (tenant_id, alert_type, severity, source_ip, username, message, raw_log, created_at)
		 VALUES ($1, $2, $3, NULLIF($4,'')::inet, $5, $6, $7, $8)`,
		alert.TenantID, alert.Type, alert.Severity,
		alert.SourceIP, alert.Username, alert.Message, alert.RawLog, alert.Timestamp,
	)
	if err != nil {
		fmt.Println("Alert DB hatası:", err)
	}
}

// ---------------------------------------------------------
// TENANT DOĞRULAMA (API KEY)
// ---------------------------------------------------------

// hashAPIKey: API Key'i SHA-256 ile hashler.
func hashAPIKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

// getTenantByAPIKey: DB'den API key'e göre tenant'ı döner.
// DB yoksa (geliştirme modu) demo tenant döner.
func getTenantByAPIKey(apiKey string) (*Tenant, bool) {
	// Geliştirme modu: DB bağlantısı yoksa demo tenant
	if db == nil {
		if apiKey == "dev-api-key-12345" {
			return &Tenant{
				ID:    "00000000-0000-0000-0000-000000000001",
				Name:  "Demo Şirketi",
				Email: "demo@securestream.dev",
				Plan:  "pro",
			}, true
		}
		return nil, false
	}

	hashed := hashAPIKey(apiKey)
	var t Tenant
	err := db.QueryRow(
		`SELECT id, name, email, plan FROM tenants WHERE api_key = $1 AND is_active = TRUE`,
		hashed,
	).Scan(&t.ID, &t.Name, &t.Email, &t.Plan)
	if err != nil {
		return nil, false
	}
	return &t, true
}

// ---------------------------------------------------------
// AUTH MIDDLEWARE
// ---------------------------------------------------------

func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			// Query param olarak da kabul et (WebSocket için)
			apiKey = c.Query("api_key")
		}

		if apiKey == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "API Key gerekli (X-API-Key header)"})
			c.Abort()
			return
		}

		tenant, ok := getTenantByAPIKey(strings.TrimSpace(apiKey))
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "Geçersiz API Key"})
			c.Abort()
			return
		}

		c.Set("tenant", tenant)
		c.Next()
	}
}

// ---------------------------------------------------------
// BROADCASTER — WebSocket Yayıncı
// ---------------------------------------------------------

func broadcastToTenant(tenantID string, msg interface{}) {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	conns, ok := clients[tenantID]
	if !ok {
		return
	}
	var alive []*websocket.Conn
	for _, conn := range conns {
		if err := conn.WriteJSON(msg); err != nil {
			conn.Close()
		} else {
			alive = append(alive, conn)
		}
	}
	clients[tenantID] = alive
}

func handleMessages() {
	for {
		select {
		case alert := <-alertQueue:
			broadcastToTenant(alert.TenantID, alert)
		case flow := <-flowQueue:
			broadcastToTenant(flow.TenantID, flow)
		}
	}
}

// ---------------------------------------------------------
// UPTIME MONITOR (FAZ 4.5)
// ---------------------------------------------------------
func uptimeMonitor() {
	for {
		time.Sleep(10 * time.Second)

		// Topolojideki gerçek node'lardan uptime verisi üret
		topologyMu.RLock()
		nodes := currentTopology.Nodes
		topologyMu.RUnlock()

		uptimeDataMu.Lock()
		for _, n := range nodes {
			if n.NodeType == "function" {
				continue // Fonksiyon nodlarını uptime'a ekleme
			}
			lat := rand.Intn(40) + 5
			status := "up"
			roll := rand.Intn(100)
			if roll < 1 {
				status = "down"
				lat = 0
			} else if roll < 6 {
				status = "degraded"
				lat += 200
			}
			if existing, ok := uptimeData[n.Name]; ok {
				existing.Latency = lat
				existing.Status = status
			} else {
				uptimeData[n.Name] = &UptimeStatus{Service: n.Name, Status: status, Latency: lat, Uptime: 99.9}
			}
		}
		uptimeDataMu.Unlock()
	}
}

// ---------------------------------------------------------
// APACHE KAFKA INTEGRATION (Gerçek Veri Akışı ve Mesajlaşma)
// ---------------------------------------------------------

type KafkaLogEvent struct {
	MsgType   string    `json:"msg_type"` // "log"
	Source    string    `json:"source"`   // "kafka"
	RawLog    string    `json:"raw_log"`
	Timestamp time.Time `json:"timestamp"`
}

func initKafka() {
	addr := os.Getenv("KAFKA_BROKERS")
	if addr != "" {
		kafkaAddr = addr
	}
	fmt.Printf("📡 Connecting to Real Apache Kafka Broker at: %s...\n", kafkaAddr)

	// Kafka Writer
	kafkaWriter = &kafka.Writer{
		Addr:     kafka.TCP(kafkaAddr),
		Balancer: &kafka.LeastBytes{},
		Async:    true,
	}

	// Gerçek Kafka Tüketici (Consumer) döngülerini her başlık için arka planda başlat
	go startRealKafkaConsumer("log-ingest")
	go startRealKafkaConsumer("metrics-pipeline")
	go startRealKafkaConsumer("threat-alerts")
	go startRealKafkaConsumer("telemetry-data")
}

func publishToKafka(topic string, key string, value []byte) {
	if kafkaWriter == nil {
		return
	}
	err := kafkaWriter.WriteMessages(context.Background(), kafka.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: value,
	})
	if err != nil {
		fmt.Printf("⚠️  Kafka Publish Hatası (Topic: %s): %v\n", topic, err)
	}
}

func startRealKafkaConsumer(topic string) {
	fmt.Printf("📥 [%s] Gerçek Kafka Consumer başlatıldı...\n", topic)
	
	// Bağlantı kopmalarına karşı otomatik retry yapısı kuruyoruz
	var reader *kafka.Reader
	defer func() {
		if reader != nil {
			reader.Close()
		}
	}()

	for {
		if reader == nil {
			reader = kafka.NewReader(kafka.ReaderConfig{
				Brokers:  []string{kafkaAddr},
				Topic:    topic,
				GroupID:  "securestream-consumer-group",
				MaxBytes: 10e6, // 10MB
			})
		}

		m, err := reader.ReadMessage(context.Background())
		if err != nil {
			fmt.Printf("⚠️  Kafka Okuma Hatası (%s): %v. 3 saniye sonra yeniden bağlanılıyor...\n", topic, err)
			reader.Close()
			reader = nil
			time.Sleep(3 * time.Second)
			continue
		}

		// Aktif tenant bul (WebSocket yayını için)
		tenantID := "default"
		clientsMu.Lock()
		for tid := range clients {
			tenantID = tid
			break
		}
		clientsMu.Unlock()

		// Mesaj detaylarını hazırla
		var logMsg string
		var producerNode = string(m.Key)
		if producerNode == "" {
			producerNode = "backend"
		}

		if topic == "log-ingest" {
			var entry LogEntry
			if err := json.Unmarshal(m.Value, &entry); err == nil {
				// Ham log metnini normal işleme pipeline'ına gönder
				select {
				case logQueue <- entry:
				default:
				}
				logMsg = fmt.Sprintf("📬 [KAFKA Broker] Topic: \"%s\" | Partition: %d | Offset: %d | Log: %s", 
					topic, m.Partition, m.Offset, entry.RawLog)
			} else {
				logMsg = fmt.Sprintf("📬 [KAFKA Broker] Topic: \"%s\" | Partition: %d | Offset: %d | Payload: %s", 
					topic, m.Partition, m.Offset, string(m.Value))
			}
		} else {
			logMsg = fmt.Sprintf("📬 [KAFKA Broker] Topic: \"%s\" | Partition: %d | Offset: %d | Producer: \"%s\" | Payload: %s", 
				topic, m.Partition, m.Offset, producerNode, string(m.Value))
		}

		// 1. Görsel Akış: Producer -> Kafka Broker
		select {
		case flowQueue <- FlowEvent{
			MsgType:  "flow",
			TenantID: tenantID,
			Source:   producerNode,
			Target:   "kafka",
		}:
		default:
		}

		// 2. Görsel Akış: Kafka Broker -> Consumer Node (Veritabanı veya İşleyici)
		var consumerNode = "backend"
		if topic == "telemetry-data" {
			consumerNode = "redis"
		} else if topic == "threat-alerts" {
			consumerNode = "postgres"
		}
		select {
		case flowQueue <- FlowEvent{
			MsgType:  "flow",
			TenantID: tenantID,
			Source:   "kafka",
			Target:   consumerNode,
		}:
		default:
		}

		// WebSocket ile canlı logu yayınla
		logEv := KafkaLogEvent{
			MsgType:   "log",
			Source:    "kafka",
			RawLog:    logMsg,
			Timestamp: time.Now(),
		}
		broadcastToTenant(tenantID, logEv)
	}
}

// startKafkaBrokerSimulator: UI'da buton açıldığında gerçek Kafka'ya yüksek hızlı trafik yazan üretici döngüsü
func startKafkaBrokerSimulator() {
	topics := []string{"metrics-pipeline", "threat-alerts", "telemetry-data"}
	payloads := []string{
		`{"event":"metric_sync","cpu_avg":42.5,"memory_mb":512}`,
		`{"event":"ip_scan_complete","threats_found":0}`,
		`{"event":"db_checkpoint","rows_updated":234}`,
	}

	for {
		time.Sleep(time.Duration(rand.Intn(700)+350) * time.Millisecond) // Yüksek hızlı canlı Kafka akışı

		kafkaMu.Lock()
		active := kafkaStreamEnabled
		kafkaMu.Unlock()

		if !active {
			continue
		}

		topologyMu.Lock()
		nodes := currentTopology.Nodes
		topologyMu.Unlock()

		if len(nodes) == 0 {
			continue
		}

		// Producer düğümünü seç
		var producer = "backend"
		if len(nodes) > 1 {
			pIdx := rand.Intn(len(nodes))
			producer = nodes[pIdx].ID
			if producer == "kafka" {
				producer = "backend"
			}
		}

		// Gerçek Kafka Broker'a yaz!
		topic := topics[rand.Intn(len(topics))]
		payload := payloads[rand.Intn(len(payloads))]
		
		publishToKafka(topic, producer, []byte(payload))
	}
}

// ---------------------------------------------------------
// DATABASE BAĞLANTISI
// ---------------------------------------------------------

func initDB() {
	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}
	dsn := fmt.Sprintf("host=%s port=5432 user=securestream password=securestream_pass dbname=securestream sslmode=disable", dbHost)
	var err error
	db, err = sql.Open("postgres", dsn)
	if err != nil {
		fmt.Println("⚠️  DB bağlantısı kurulamadı, geliştirme modunda devam ediliyor:", err)
		db = nil
		return
	}
	if err = db.Ping(); err != nil {
		fmt.Println("⚠️  DB'ye ulaşılamıyor, geliştirme modunda devam ediliyor:", err)
		db = nil
		return
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	fmt.Println("✅ PostgreSQL bağlantısı kuruldu.")

	// Otomatik Tablo Oluşturma (Database Auto-Migrations)
	fmt.Println("🔄 Veritabanı tabloları kontrol ediliyor/oluşturuluyor...")
	schema := `
	CREATE EXTENSION IF NOT EXISTS "pgcrypto";

	CREATE TABLE IF NOT EXISTS tenants (
		id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		name        VARCHAR(255) NOT NULL,
		email       VARCHAR(255) NOT NULL UNIQUE,
		api_key     VARCHAR(64)  NOT NULL UNIQUE, -- SHA-256 hash
		plan        VARCHAR(50)  NOT NULL DEFAULT 'free',
		is_active   BOOLEAN      NOT NULL DEFAULT TRUE,
		created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
		updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS log_entries (
		id          BIGSERIAL    PRIMARY KEY,
		tenant_id   UUID         NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
		source      VARCHAR(100) NOT NULL,
		raw_log     TEXT         NOT NULL,
		source_ip   INET,
		username    VARCHAR(255),
		created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_log_entries_tenant ON log_entries(tenant_id, created_at DESC);
	CREATE INDEX IF NOT EXISTS idx_log_entries_ip ON log_entries(source_ip);

	CREATE TABLE IF NOT EXISTS alerts (
		id          BIGSERIAL    PRIMARY KEY,
		tenant_id   UUID         NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
		alert_type  VARCHAR(100) NOT NULL,
		severity    VARCHAR(20)  NOT NULL,
		source_ip   INET,
		username    VARCHAR(255),
		message     TEXT         NOT NULL,
		raw_log     TEXT,
		acknowledged BOOLEAN     NOT NULL DEFAULT FALSE,
		created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_alerts_tenant ON alerts(tenant_id, created_at DESC);
	CREATE INDEX IF NOT EXISTS idx_alerts_severity ON alerts(tenant_id, severity);
	CREATE INDEX IF NOT EXISTS idx_alerts_type ON alerts(tenant_id, alert_type);

	-- Demo API Anahtarı "dev-api-key-12345" SHA-256 hash'i ile tohumlanıyor
	INSERT INTO tenants (id, name, email, api_key, plan)
	VALUES (
		'00000000-0000-0000-0000-000000000001',
		'Demo Şirketi A.Ş.',
		'demo@securestream.dev',
		'8264dc9f07e749d9c2ffead0b25de8cb22bed7af774e189ef224ae015908776b',
		'pro'
	) ON CONFLICT DO NOTHING;
	`
	_, err = db.Exec(schema)
	if err != nil {
		fmt.Printf("⚠️  Otomatik veritabanı kurulumu başarısız oldu: %v\n", err)
	} else {
		fmt.Println("✅ Veritabanı tabloları ve demo API anahtarı başarıyla kuruldu.")
	}
}

// ---------------------------------------------------------
// REDIS BAĞLANTISI
// ---------------------------------------------------------

func initRedis() {
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = "localhost"
	}
	rdb = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:6379", redisHost),
		Password: "",
		DB:       0,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		fmt.Println("⚠️  Redis'e ulaşılamıyor, rate limiting devre dışı:", err)
		rdb = nil
		return
	}
	fmt.Println("✅ Redis bağlantısı kuruldu.")
}

// ---------------------------------------------------------
// RATE LIMIT MIDDLEWARE (Redis sliding window)
// ---------------------------------------------------------
// Kural: Her tenant için dakikada max 60 istek (ingest için 30)

func rateLimitMiddleware(maxPerMinute int) gin.HandlerFunc {
	return func(c *gin.Context) {
		if rdb == nil {
			c.Next()
			return
		}
		tenant, exists := c.Get("tenant")
		if !exists {
			c.Next()
			return
		}
		t := tenant.(*Tenant)
		key := fmt.Sprintf("rl:%s:%d", t.ID, time.Now().Unix()/60)
		ctx := context.Background()

		pipe := rdb.Pipeline()
		incr := pipe.Incr(ctx, key)
		pipe.Expire(ctx, key, 90*time.Second)
		_, err := pipe.Exec(ctx)
		if err != nil {
			c.Next()
			return
		}
		count := incr.Val()
		c.Header("X-RateLimit-Limit", strconv.Itoa(maxPerMinute))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(max(0, maxPerMinute-int(count))))

		if int(count) > maxPerMinute {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": fmt.Sprintf("Rate limit aşıldı: dakikada maksimum %d istek", maxPerMinute),
				"retry_after": 60 - time.Now().Second(),
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ---------------------------------------------------------
// ANA FONKSİYON
// ---------------------------------------------------------

// ---------------------------------------------------------
// JARVIS AI MOTORU
// ---------------------------------------------------------

type JarvisRequest struct {
	Message string `json:"message"`
}

type JarvisResponse struct {
	Reply  string      `json:"reply"`
	Action string      `json:"action,omitempty"`
	Data   interface{} `json:"data,omitempty"`
}

func containsAny(s string, keywords ...string) bool {
	for _, k := range keywords {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type LlamaResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func askLlamaAPI(prompt string) (JarvisResponse, error) {
	apiKey := os.Getenv("GROQ_API_KEY")
	if apiKey == "" {
		return JarvisResponse{}, fmt.Errorf("no api key")
	}

	reqBody, _ := json.Marshal(map[string]interface{}{
		"model": "llama3-8b-8192",
		"messages": []map[string]string{
			{"role": "system", "content": `You are J.A.R.V.I.S, an AI security assistant for SecureStream. Respond in English ONLY. 
You receive a user message and system context.
If the user explicitly wants to block an IP address, reply STRICTLY with JSON format: {"action":"block_ip", "ip":"<IP>", "reply":"I have blocked the IP <IP>."}
If asking for status, alerts, topology, or general questions, reply STRICTLY with JSON format: {"action":"status", "reply":"<your conversational English response based on context>"}
Do not output anything outside the JSON structure.`},
			{"role": "user", "content": prompt},
		},
		"response_format": map[string]string{"type": "json_object"},
	})

	req, _ := http.NewRequest("POST", "https://api.groq.com/openai/v1/chat/completions", bytes.NewBuffer(reqBody))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 7 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return JarvisResponse{}, err
	}
	defer resp.Body.Close()

	var llamaResp LlamaResponse
	json.NewDecoder(resp.Body).Decode(&llamaResp)

	if len(llamaResp.Choices) == 0 {
		return JarvisResponse{}, fmt.Errorf("no response from llama")
	}

	content := llamaResp.Choices[0].Message.Content
	var jResp struct {
		Action string `json:"action"`
		Reply  string `json:"reply"`
		IP     string `json:"ip"`
	}
	err = json.Unmarshal([]byte(content), &jResp)
	if err != nil {
		return JarvisResponse{Reply: content}, nil
	}

	ret := JarvisResponse{Reply: jResp.Reply, Action: jResp.Action}
	if jResp.IP != "" {
		ret.Data = map[string]string{"ip": jResp.IP}
	}
	return ret, nil
}

func jarvisBrain(message string) JarvisResponse {
	msg := strings.ToLower(message)
	ipRe := regexp.MustCompile(`\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`)

	topologyMu.RLock()
	nodeCount := len(currentTopology.Nodes)
	linkCount := len(currentTopology.Links)
	topologyMu.RUnlock()

	blockedIPsMu.RLock()
	blockedCount := len(blockedIPs)
	blockedIPsMu.RUnlock()

	recentAlertsMu.RLock()
	alertSnap := make([]Alert, len(recentAlertsMem))
	copy(alertSnap, recentAlertsMem)
	recentAlertsMu.RUnlock()

	critCount, highCount := 0, 0
	for _, a := range alertSnap {
		if a.Severity == "critical" {
			critCount++
		}
		if a.Severity == "high" {
			highCount++
		}
	}

	// Llama Entegrasyonu
	systemContext := fmt.Sprintf("CONTEXT: %d nodes, %d links, %d alerts (%d critical), %d blocked IPs.", nodeCount, linkCount, len(alertSnap), critCount, blockedCount)
	fullPrompt := systemContext + "\nUSER MESSAGE: " + message
	
	llmResp, err := askLlamaAPI(fullPrompt)
	if err == nil && llmResp.Reply != "" {
		// Eğer LLM action olarak block_ip döndürdüyse arka planda engelle
		if llmResp.Action == "block_ip" {
			if dataMap, ok := llmResp.Data.(map[string]string); ok && dataMap["ip"] != "" {
				blockedIPsMu.Lock()
				blockedIPs[dataMap["ip"]] = true
				blockedIPsMu.Unlock()
			}
		}
		return llmResp
	}

	// FALLBACK: Kural Tabanlı (Eğer GROQ_API_KEY yoksa)
	// Intent: Block
	if containsAny(msg, "block", "ban") {
		ip := ipRe.FindString(message)
		if ip != "" {
			blockedIPsMu.Lock()
			blockedIPs[ip] = true
			blockedIPsMu.Unlock()
			return JarvisResponse{
				Reply:  fmt.Sprintf("Understood. The IP address %s has been added to the blocklist. All requests from this source will now be denied.", ip),
				Action: "block_ip",
				Data:   map[string]string{"ip": ip},
			}
		}
		return JarvisResponse{Reply: "Please specify the IP address you want to block. Example: 'Block the IP 185.220.101.5'"}
	}

	// Intent: Blocked list
	if containsAny(msg, "blocked", "ban list") {
		blockedIPsMu.RLock()
		ips := make([]string, 0, len(blockedIPs))
		for ip := range blockedIPs {
			ips = append(ips, ip)
		}
		blockedIPsMu.RUnlock()
		if len(ips) == 0 {
			return JarvisResponse{Reply: "The blocked IP list is currently empty. The system is open to all sources."}
		}
		return JarvisResponse{
			Reply: fmt.Sprintf("%d IPs are blocked: %s", len(ips), strings.Join(ips[:minInt(5, len(ips))], ", ")),
			Data:  ips,
		}
	}

	// Intent: Alerts
	if containsAny(msg, "alert", "attack", "brute", "hack", "security") {
		if len(alertSnap) == 0 {
			return JarvisResponse{Reply: "There are no active security threats. The system is operating nominally."}
		}
		var lines []string
		for _, a := range alertSnap {
			if a.Severity == "critical" || a.Severity == "high" {
				lines = append(lines, fmt.Sprintf("[%s] %s", strings.ToUpper(a.Severity), a.Message))
			}
			if len(lines) >= 4 {
				break
			}
		}
		if len(lines) == 0 {
			return JarvisResponse{Reply: fmt.Sprintf("There are %d low-level alerts. No critical threats.", len(alertSnap))}
		}
		return JarvisResponse{
			Reply:  fmt.Sprintf("Recent high-priority alerts:\n%s", strings.Join(lines, "\n")),
			Action: "show_alerts",
			Data:   alertSnap[:minInt(5, len(alertSnap))],
		}
	}

	// Intent: Topology
	if containsAny(msg, "topology", "service", "node", "network") {
		topologyMu.RLock()
		parentCount := 0
		for _, n := range currentTopology.Nodes {
			if n.Parent == "" {
				parentCount++
			}
		}
		topologyMu.RUnlock()
		return JarvisResponse{
			Reply:  fmt.Sprintf("Monitoring %d main services, a total of %d nodes, and %d active links.", parentCount, nodeCount, linkCount),
			Action: "topology",
		}
	}

	// Intent: Status
	if containsAny(msg, "status", "health", "summary", "hello", "hi") {
		status := "GREEN — nominal"
		if critCount > 0 {
			status = "RED — critical alert active"
		} else if highCount > 0 {
			status = "YELLOW — high priority warning"
		}
		return JarvisResponse{
			Reply: fmt.Sprintf(
				"System Status: %s\n• %d nodes, %d links active\n• %d alerts (%d critical, %d high)\n• %d IPs blocked",
				status, nodeCount, linkCount, len(alertSnap), critCount, highCount, blockedCount,
			),
			Action: "status",
		}
	}

	// Intent: Help
	if containsAny(msg, "help", "command") {
		return JarvisResponse{
			Reply: "Available commands:\n• system status\n• recent alerts\n• block [IP]\n• topology status\n• blocked IPs",
		}
	}

	return JarvisResponse{
		Reply: fmt.Sprintf(
			"I didn't catch that. Currently tracking %d nodes and %d alerts. Type 'help' for commands.",
			nodeCount, len(alertSnap),
		),
	}
}

func main() {
	// DB ve Redis bağlantıları
	initDB()
	initRedis()
	initKafka()

	r := gin.Default()

	// CORS
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// Worker Pool başlat (5 worker)
	var wg sync.WaitGroup
	for i := 1; i <= 5; i++ {
		wg.Add(1)
		go worker(i, &wg)
	}

	// Broadcaster başlat
	go handleMessages()

	// Uptime monitor başlat
	go uptimeMonitor()

	// Kafka Broker simülatörünü başlat
	go startKafkaBrokerSimulator()

	// -------------------------------------------------------
	// PUBLIC ENDPOINTS
	// -------------------------------------------------------

	// Sistem sağlık kontrolü (Faz 2: Redis durumu da dahil)
	r.GET("/health", func(c *gin.Context) {
		dbOK := db != nil
		redisOK := false
		if rdb != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()
			redisOK = rdb.Ping(ctx).Err() == nil
		}
		status := "ok"
		if !dbOK {
			status = "degraded"
		}
		c.JSON(http.StatusOK, gin.H{
			"status":    status,
			"service":   "SecureStream",
			"db":        dbOK,
			"redis":     redisOK,
			"timestamp": time.Now().UTC(),
		})
	})

	// -------------------------------------------------------
	// AWS MARKETPLACE REGISTER & RESOLUTION ENDPOINT
	// -------------------------------------------------------
	// AWS Marketplace müşterileri satın alımdan sonra bu endpoint'e yönlendirilir.
	// Örn: GET /register?token=x-amzn-marketplace-token
	r.GET("/register", func(c *gin.Context) {
		token := c.Query("token")
		if token == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing registration token"})
			return
		}

		// Akıllı Mock Modu Tespiti:
		// Ortamda AWS kimlik bilgileri tanımlı değilse veya gelen token "siber_"/"mock_" ile başlıyorsa Mock modu çalıştır.
		hasAWSCreds := os.Getenv("AWS_ACCESS_KEY_ID") != "" || 
		               os.Getenv("AWS_ROLE_ARN") != "" || 
		               os.Getenv("AWS_CONTAINER_CREDENTIALS_RELATIVE_URI") != ""
		
		isMockToken := strings.HasPrefix(token, "siber_") || strings.HasPrefix(token, "mock_")

		if !hasAWSCreds || isMockToken {
			fmt.Println("ℹ️  [AWS Billing Mock] AWS credentials not found or Mock Token detected. Bypassing real AWS API Call.")
			
			// Mock modda yeni bir B2B Tenant kaydı oluşturur
			newTenantKey := "sk_live_" + strings.ToLower(token[:12])
			hashedKey := hashAPIKey(newTenantKey)
			
			// Tenant tablosuna doğrudan SHA-256 hash'li API anahtarı ile kaydet (ID otomatik üretilir)
			_, err := db.Exec("INSERT INTO tenants (name, email, api_key, plan) VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING", 
				"AWS Marketplace Customer (Mock)", "aws-client-" + token[:8] + "@securestream.ai", hashedKey, "enterprise")
			if err != nil {
				fmt.Printf("⚠️  [AWS Billing Mock] Failed to insert mock tenant: %v\n", err)
			}

			c.JSON(http.StatusOK, gin.H{
				"status": "success",
				"mode": "MOCK",
				"message": "AWS Marketplace Mock Customer resolved successfully",
				"customer_id": "mock-customer-" + token[:8],
				"api_key": newTenantKey,
			})
			return
		}

		awsClient, err := NewAWSMarketplaceClient()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("AWS SDK Client Init failed: %v", err)})
			return
		}

		// Gerçek AWS API Call
		customerID, productCode, err := awsClient.ResolveCustomer(token)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("AWS Token Resolution failed: %v", err)})
			return
		}

		// Yeni tenant oluştur ve veritabanına ekle
		newTenantKey := "sk_live_" + strings.ToLower(customerID[:12])
		hashedKey := hashAPIKey(newTenantKey)

		_, err = db.Exec("INSERT INTO tenants (name, email, api_key, plan) VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING", 
			"AWS Marketplace (" + productCode + ")", "aws-client-" + customerID[:8] + "@securestream.ai", hashedKey, "enterprise")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create tenant"})
			return
		}

		// Saatlik otomatik fatura bildirme worker'ını arka planda başlat
		StartAWSUsageReporting(awsClient, productCode)

		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"mode": "PRODUCTION",
			"message": "AWS Marketplace Customer registered successfully",
			"customer_id": customerID,
			"api_key": newTenantKey,
		})
	})

	// -------------------------------------------------------
	// AUTHENTICATED ENDPOINTS (API Key zorunlu)
	// -------------------------------------------------------
	
	// Gerçek zamanlı API Trafik Takibi (Gerçek Veri Akışı)
	r.Use(func(c *gin.Context) {
		// Sadece authenticated endpointlerde tenant bilgisi olur
		if tenant, exists := c.Get("tenant"); exists {
			t := tenant.(*Tenant)
			source := "internet"
			if strings.Contains(c.Request.Referer(), "localhost") || strings.Contains(c.Request.Referer(), "3000") {
				source = "frontend"
			}
			select {
			case flowQueue <- FlowEvent{
				MsgType:  "flow",
				TenantID: t.ID,
				Source:   source,
				Target:   "backend",
			}:
			default:
			}
		}
		c.Next()
	})

	api := r.Group("/api", authMiddleware())

	// WebSocket bağlantısı — canlı alert akışı
	api.GET("/ws", func(c *gin.Context) {
		tenant := c.MustGet("tenant").(*Tenant)

		ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			fmt.Println("WS Hatası:", err)
			return
		}

		clientsMu.Lock()
		clients[tenant.ID] = append(clients[tenant.ID], ws)
		clientsMu.Unlock()

		fmt.Printf("🔌 Yeni bağlantı: %s (Tenant: %s)\n", ws.RemoteAddr(), tenant.Name)
	})

	// Dinamik 3D Topoloji endpoint'i
	api.GET("/topology", func(c *gin.Context) {
		topologyMu.RLock()
		defer topologyMu.RUnlock()
		
		topo := currentTopology
		if topo.Nodes == nil {
			topo.Nodes = []TopologyNode{}
		}
		if topo.Links == nil {
			topo.Links = []TopologyLink{}
		}

		kafkaMu.Lock()
		active := kafkaStreamEnabled
		kafkaMu.Unlock()

		if active {
			hasKafka := false
			for i, n := range topo.Nodes {
				if n.ID == "kafka" {
					topo.Nodes[i].Name = "Apache Kafka Cluster"
					topo.Nodes[i].Tech = "Apache Kafka"
					topo.Nodes[i].NodeType = "message-broker"
					topo.Nodes[i].Color = "#f97316" // Premium Kafka Turuncusu
					topo.Nodes[i].Val = 8
					topo.Nodes[i].Group = 3
					hasKafka = true
					break
				}
			}
			if !hasKafka {
				topo.Nodes = append(topo.Nodes, TopologyNode{
					ID:        "kafka",
					Name:      "Apache Kafka Cluster",
					Tech:      "Apache Kafka",
					NodeType:  "message-broker",
					Color:     "#f97316", // Kafka Turuncusu
					Val:       8,
					Group:     3,
				})
			}

			// Aktif servisleri Kafka'ya Pub/Sub olarak bağlayan linkleri HER DURUMDA ekle!
			for _, n := range topo.Nodes {
				if n.ID == "kafka" || n.NodeType == "function" {
					continue
				}
				// docker-compose.yml parse hatalarından sızan sahte parametre düğümlerini bağlamıyoruz
				if n.ID == "depends_on" || n.ID == "environment" || n.ID == "volumes" || n.ID == "aliases" || n.ID == "healthcheck" || n.ID == "networks" || n.ID == "ports" || n.ID == "build" || n.ID == "default" {
					continue
				}

				topo.Links = append(topo.Links, TopologyLink{
					Source: n.ID,
					Target: "kafka",
					Val:    2,
					Label:  "Publish",
				})
				topo.Links = append(topo.Links, TopologyLink{
					Source: "kafka",
					Target: n.ID,
					Val:    2,
					Label:  "Subscribe",
				})
			}
		}
		c.JSON(http.StatusOK, topo)
	})

	// Kafka Status API
	api.GET("/kafka/status", func(c *gin.Context) {
		kafkaMu.Lock()
		active := kafkaStreamEnabled
		kafkaMu.Unlock()
		
		status := "disabled"
		if active {
			status = "enabled"
		}
		c.JSON(http.StatusOK, gin.H{"status": "success", "kafka_stream": status})
	})

	// Kafka Toggle API
	api.POST("/kafka/toggle", func(c *gin.Context) {
		var req struct {
			Enabled bool `json:"enabled"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		
		kafkaMu.Lock()
		kafkaStreamEnabled = req.Enabled
		kafkaMu.Unlock()
		
		status := "disabled"
		if req.Enabled {
			status = "enabled"
		}
		c.JSON(http.StatusOK, gin.H{"status": "success", "kafka_stream": status})
	})

	// Topolojiyi güncelleme endpoint'i
	api.POST("/topology", func(c *gin.Context) {
		var req TopologyData
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		
		if req.Nodes == nil {
			req.Nodes = []TopologyNode{}
		}
		if req.Links == nil {
			req.Links = []TopologyLink{}
		}
		
		topologyMu.Lock()
		// Akıllı MERGE: Yeni taranan dizinlerin verilerini eskilerle birleştiriyoruz.
		// Böylece birden fazla proje taranırken eski projelerin düğümleri silinmez ve
		// projeler arası HTTP bağlantıları (link) aynı düğüm ID'leri üzerinden otomatik eşleşir.
		mergedNodes := make(map[string]TopologyNode)
		for _, n := range currentTopology.Nodes {
			mergedNodes[n.ID] = n
		}
		for _, n := range req.Nodes {
			// Eğer yeni düğüm zaten varsa, bilgilerini (Tech, NodeType vs.) güncelleyebiliriz
			mergedNodes[n.ID] = n
		}

		mergedLinks := make(map[string]TopologyLink)
		for _, l := range currentTopology.Links {
			key := l.Source + "|||" + l.Target
			mergedLinks[key] = l
		}
		for _, l := range req.Links {
			key := l.Source + "|||" + l.Target
			mergedLinks[key] = l
		}

		newNodes := make([]TopologyNode, 0, len(mergedNodes))
		for _, n := range mergedNodes {
			newNodes = append(newNodes, n)
		}
		newLinks := make([]TopologyLink, 0, len(mergedLinks))
		for _, l := range mergedLinks {
			newLinks = append(newLinks, l)
		}

		currentTopology = TopologyData{
			Nodes: newNodes,
			Links: newLinks,
		}
		topologyMu.Unlock()
		
		c.JSON(http.StatusOK, gin.H{"status": "topology_updated"})
	})

	// İstatistik endpoint'i
	api.GET("/stats", func(c *gin.Context) {
		tenant := c.MustGet("tenant").(*Tenant)
		if db == nil {
			c.JSON(http.StatusOK, gin.H{"total_logs": 0, "total_alerts": 0, "by_severity": gin.H{}})
			return
		}
		var totalLogs, totalAlerts int
		db.QueryRow(`SELECT COUNT(*) FROM log_entries WHERE tenant_id=$1`, tenant.ID).Scan(&totalLogs)
		db.QueryRow(`SELECT COUNT(*) FROM alerts WHERE tenant_id=$1`, tenant.ID).Scan(&totalAlerts)

		rows, _ := db.Query(
			`SELECT severity, COUNT(*) FROM alerts WHERE tenant_id=$1 GROUP BY severity`, tenant.ID,
		)
		bySeverity := gin.H{}
		if rows != nil {
			defer rows.Close()
			for rows.Next() {
				var sev string
				var cnt int
				rows.Scan(&sev, &cnt)
				bySeverity[sev] = cnt
			}
		}
		var rateLimit interface{} = nil
		if rdb != nil {
			key := fmt.Sprintf("rl:%s:%d", tenant.ID, time.Now().Unix()/60)
			val, err := rdb.Get(context.Background(), key).Int()
			if err == nil {
				rateLimit = gin.H{"used": val, "limit": 60}
			}
		}
		c.JSON(http.StatusOK, gin.H{
			"total_logs":   totalLogs,
			"total_alerts": totalAlerts,
			"by_severity":  bySeverity,
			"rate_limit":   rateLimit,
		})
	})

	// Log arama endpoint'i
	api.GET("/logs", func(c *gin.Context) {
		tenant := c.MustGet("tenant").(*Tenant)
		if db == nil {
			c.JSON(http.StatusOK, gin.H{"logs": []gin.H{}})
			return
		}
		search := strings.TrimSpace(c.Query("q"))
		source := c.Query("source")
		limitStr := c.DefaultQuery("limit", "100")
		limit, _ := strconv.Atoi(limitStr)
		if limit > 500 {
			limit = 500
		}
		query := `SELECT id, source, raw_log, COALESCE(source_ip::text,''), created_at FROM log_entries
		          WHERE tenant_id=$1`
		args := []interface{}{tenant.ID}
		argN := 2
		if search != "" {
			query += fmt.Sprintf(" AND raw_log ILIKE $%d", argN)
			args = append(args, "%"+search+"%")
			argN++
		}
		if source != "" {
			query += fmt.Sprintf(" AND source=$%d", argN)
			args = append(args, source)
			argN++
		}
		query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d", argN)
		args = append(args, limit)

		rows, err := db.Query(query, args...)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()
		var logs []gin.H
		for rows.Next() {
			var id int64
			var src, raw, ip string
			var ts time.Time
			rows.Scan(&id, &src, &raw, &ip, &ts)
			logs = append(logs, gin.H{"id": id, "source": src, "raw_log": raw, "source_ip": ip, "timestamp": ts})
		}
		if logs == nil {
			logs = []gin.H{}
		}
		c.JSON(http.StatusOK, gin.H{"logs": logs, "count": len(logs)})
	})

	// Audit Trail CSV export
	api.GET("/alerts/export", func(c *gin.Context) {
		tenant := c.MustGet("tenant").(*Tenant)
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "DB bağlantısı yok"})
			return
		}
		rows, err := db.Query(
			`SELECT id, alert_type, severity, COALESCE(source_ip::text,''), COALESCE(username,''), message, created_at
			 FROM alerts WHERE tenant_id=$1 ORDER BY created_at DESC`,
			tenant.ID,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()
		filename := fmt.Sprintf("securestream-audit-%s.csv", time.Now().Format("2006-01-02"))
		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Header("Content-Disposition", "attachment; filename="+filename)
		w := csv.NewWriter(c.Writer)
		w.Write([]string{"ID", "Tür", "Seviye", "Kaynak IP", "Kullanıcı", "Mesaj", "Zaman"})
		for rows.Next() {
			var id int64
			var alertType, severity, ip, username, message string
			var ts time.Time
			rows.Scan(&id, &alertType, &severity, &ip, &username, &message, &ts)
			w.Write([]string{
				strconv.FormatInt(id, 10), alertType, severity, ip, username, message,
				ts.Format("2006-01-02 15:04:05"),
			})
		}
		w.Flush()
	})

	// Log gönderme endpoint'i — rate limit uygulanıyor
	api.POST("/ingest", rateLimitMiddleware(60), func(c *gin.Context) {
		tenant := c.MustGet("tenant").(*Tenant)

		var req IngestRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		entry := LogEntry{
			TenantID:  tenant.ID,
			Source:    req.Source,
			RawLog:    req.RawLog,
			Timestamp: time.Now(),
		}

		// --- IP ENGELLEME DENETİMİ (Enforcement) ---
		// Log içindeki IP'yi çıkar ve engelli listesinde mi bak
		foundIP := reIP.FindString(req.RawLog)
		if foundIP != "" {
			blockedIPsMu.RLock()
			isBlocked := blockedIPs[foundIP]
			blockedIPsMu.RUnlock()

			if isBlocked {
				c.JSON(http.StatusForbidden, gin.H{
					"error":  "Security Breach Prevention",
					"ip":     foundIP,
					"message": "Traffic from this source is blocked by SecureStream JARVIS AI.",
				})
				return
			}
		}

		// Real Apache Kafka Broker'a publish et! (Veri akışı gerçek kuyruğa gider)
		entryBytes, err := json.Marshal(entry)
		if err == nil {
			publishToKafka("log-ingest", entry.Source, entryBytes)
			c.JSON(http.StatusAccepted, gin.H{"status": "published_to_kafka"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "JSON serialize hatası"})
		}
	})

	// Geçmiş alertleri getir
	api.GET("/alerts", func(c *gin.Context) {
		tenant := c.MustGet("tenant").(*Tenant)

		if db == nil {
			c.JSON(http.StatusOK, gin.H{"alerts": []Alert{}, "note": "DB bağlantısı yok"})
			return
		}

		rows, err := db.Query(
			`SELECT id, alert_type, severity, COALESCE(source_ip::text,''), 
			        COALESCE(username,''), message, created_at 
			 FROM alerts WHERE tenant_id = $1 
			 ORDER BY created_at DESC LIMIT 100`,
			tenant.ID,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		var alerts []Alert
		for rows.Next() {
			var a Alert
			rows.Scan(&a.ID, &a.Type, &a.Severity, &a.SourceIP, &a.Username, &a.Message, &a.Timestamp)
			a.TenantID = tenant.ID
			alerts = append(alerts, a)
		}
		if alerts == nil {
			alerts = []Alert{}
		}

		c.JSON(http.StatusOK, gin.H{"alerts": alerts})
	})

	// Tenant bilgisi
	api.GET("/me", func(c *gin.Context) {
		tenant := c.MustGet("tenant").(*Tenant)
		c.JSON(http.StatusOK, tenant)
	})

	// Uptime endpoint'i
	api.GET("/uptime", func(c *gin.Context) {
		uptimeDataMu.RLock()
		defer uptimeDataMu.RUnlock()
		var res []UptimeStatus
		for _, v := range uptimeData {
			res = append(res, *v)
		}
		c.JSON(http.StatusOK, gin.H{"uptime": res})
	})

	// JARVIS AI Chat endpoint'i
	api.POST("/jarvis/chat", func(c *gin.Context) {
		var req JarvisRequest
		if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Message) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "message gerekli"})
			return
		}
		resp := jarvisBrain(req.Message)
		c.JSON(http.StatusOK, resp)
	})

	// Sistem Aksiyonları endpoint'i (Engellenen IP'ler vb.)
	api.GET("/actions", func(c *gin.Context) {
		blockedIPsMu.RLock()
		defer blockedIPsMu.RUnlock()
		var ips []string
		for ip := range blockedIPs {
			ips = append(ips, ip)
		}
		c.JSON(http.StatusOK, gin.H{"blocked_ips": ips})
	})

	// Discovery Agent tetikleme endpoint'i
	// POST /api/discover {"dirs": ["/path/to/project"]}
	type DiscoverRequest struct {
		Dirs     []string `json:"dirs" binding:"required"`
		CodeOnly bool     `json:"code_only"`
	}
	type DiscoverStatus struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		Nodes   int    `json:"nodes"`
		Links   int    `json:"links"`
	}

	api.POST("/discover", func(c *gin.Context) {
		var req DiscoverRequest
		if err := c.ShouldBindJSON(&req); err != nil || len(req.Dirs) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "dirs alanı gerekli (örn: [\"/Users/me/project\"])"})
			return
		}

		// Agent binary path
		agentPath := os.Getenv("DISCOVERY_AGENT_PATH")
		if agentPath == "" {
			if _, err := os.Stat("./discovery-agent-bin"); err == nil {
				agentPath = "./discovery-agent-bin"
			} else if _, err := os.Stat("../discovery-agent-bin"); err == nil {
				agentPath = "../discovery-agent-bin"
			} else {
				agentPath = "../discovery-agent-bin"
			}
		}

		// Yeni bir tarama seansı başladığı için mevcut topolojiyi sıfırlıyoruz.
		// Böylece temiz bir sayfa açılır ve döngüdeki her tarama verisi birbiriyle birleşir.
		topologyMu.Lock()
		currentTopology = TopologyData{
			Nodes: []TopologyNode{
				{ID: "internet", Name: "Internet (External)", Val: 10, Group: 1, Color: "#94a3b8", NodeType: "edge"},
			},
			Links: []TopologyLink{},
		}
		topologyMu.Unlock()

		// Docker ortamında olup olmadığımızı /host-project klasörünün varlığıyla anlıyoruz
		inDocker := false
		if _, err := os.Stat("/host-project"); err == nil {
			inDocker = true
		}

		for _, dir := range req.Dirs {
			// Eğer Docker'dayız ve taranacak dizin host'taki proje klasörümüzü işaret ediyorsa,
			// bunu Docker içindeki karşılığı olan "/host-project" olarak yeniden eşleştiriyoruz.
			if inDocker && (dir == "/Users/niyazimertisiksal/Concurrent-Log-Streamer" || strings.HasPrefix(dir, "/Users/niyazimertisiksal/Concurrent-Log-Streamer/")) {
				oldDir := dir
				dir = strings.Replace(dir, "/Users/niyazimertisiksal/Concurrent-Log-Streamer", "/host-project", 1)
				fmt.Printf("🐳 Docker Path Mapping: %s -> %s\n", oldDir, dir)
			}

			fmt.Printf("🔍 Discovery Agent başlatılıyor: %s\n", dir)
			
			// discovery-agent'ı çalıştır
			// Bu agent, -dir içindeki kodu tarar ve -backend'e POST /api/topology isteği yapar.
			args := []string{"-dir", dir, "-backend", "http://localhost:8080/api"}
			if req.CodeOnly {
				args = append(args, "-code-only")
			}
			cmd := exec.Command(agentPath, args...)
			
			// Output'u yakala (opsiyonel hata ayıklama için)
			var out bytes.Buffer
			var stderr bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &stderr

			err := cmd.Run()
			if err != nil {
				fmt.Printf("⚠️ Agent çalışma hatası (%s): %s\n", dir, err.Error())
				fmt.Printf("Agent Stderr: %s\n", stderr.String())
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Agent çalıştırılamadı: " + err.Error()})
				return
			}
			fmt.Printf("Agent Çıktısı: %s\n", out.String())
		}

		// Topolojinin POST /api/topology üzerinden gelmesi bekleniyor
		// Biz sadece success döneceğiz
		c.JSON(http.StatusOK, DiscoverStatus{
			Status:  "completed",
			Message: fmt.Sprintf("%d dizin tarandı ve topoloji backend'e aktarıldı", len(req.Dirs)),
			Nodes:   0, // Client side yeniden fetch edecek
			Links:   0,
		})
	})

	fmt.Println("🚀 SecureStream hazır → http://localhost:8080")
	r.Run(":8080")
}
