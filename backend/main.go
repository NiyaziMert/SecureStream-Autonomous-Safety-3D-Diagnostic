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
	ID     string `json:"id"`
	Name   string `json:"name"`
	Val    int    `json:"val"`
	Group  int    `json:"group"`
	Color  string `json:"color"`
	Parent string `json:"parent,omitempty"`
}

type TopologyLink struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Val    int    `json:"val"`
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
	uptimeData   = make(map[string]*UptimeStatus)
	uptimeDataMu sync.RWMutex
)

// FlowEvent: Gerçek zamanlı veri akışı olayı
type FlowEvent struct {
	MsgType  string `json:"msg_type"` // "flow"
	TenantID string `json:"-"`
	Source   string `json:"source"`
	Target   string `json:"target"`
}

// flowMap: Log kaynağını topoloji bağlantılarına eşleyen harita
var flowMap = map[string][][2]string{
	"internet":            {{"internet", "waf"}},
	"waf":                 {{"internet", "waf"}, {"waf", "nginx"}},
	"nginx":               {{"waf", "nginx"}, {"nginx", "auth_service"}},
	"auth_service":        {{"nginx", "auth_service"}, {"auth_service", "db_master"}},
	"product_service":     {{"nginx", "product_service"}, {"product_service", "redis"}, {"product_service", "db_replica"}},
	"payment_service":     {{"nginx", "payment_service"}, {"payment_service", "pay_fraud"}},
	"notification_service": {{"nginx", "notification_service"}},
	"pay_stripe":          {{"payment_service", "pay_stripe"}, {"pay_stripe", "db_master"}},
	"pay_paypal":          {{"payment_service", "pay_paypal"}, {"pay_paypal", "db_master"}},
	"pay_fraud":           {{"payment_service", "pay_fraud"}, {"pay_fraud", "db_master"}},
	"pay_invoice":         {{"payment_service", "pay_invoice"}, {"pay_invoice", "db_master"}},
	"auth_jwt":            {{"auth_service", "auth_jwt"}, {"auth_jwt", "redis"}},
	"auth_oauth":          {{"auth_service", "auth_oauth"}, {"auth_oauth", "db_master"}},
	"auth_session":        {{"auth_service", "auth_session"}, {"auth_session", "redis"}},
	"prod_search":         {{"product_service", "prod_search"}, {"prod_search", "elasticsearch"}},
	"prod_inventory":      {{"product_service", "prod_inventory"}, {"prod_inventory", "db_replica"}},
	"prod_recommend":      {{"product_service", "prod_recommend"}, {"prod_recommend", "redis"}},
	"fn_stripe_validate":  {{"pay_stripe", "fn_stripe_validate"}},
	"fn_stripe_charge":    {{"fn_stripe_validate", "fn_stripe_charge"}, {"fn_stripe_charge", "db_master"}},
	"fn_stripe_refund":    {{"fn_stripe_validate", "fn_stripe_refund"}, {"fn_stripe_refund", "db_master"}},
	"fn_stripe_webhook":   {{"fn_stripe_charge", "fn_stripe_webhook"}},
	"db_master":           {{"auth_service", "db_master"}, {"db_master", "db_replica"}},
	"db_replica":          {{"db_master", "db_replica"}},
	"redis":               {{"product_service", "redis"}, {"auth_service", "redis"}},
	"elasticsearch":       {{"prod_search", "elasticsearch"}},
}

func mapLogToFlows(entry LogEntry) []FlowEvent {
	pairs, ok := flowMap[entry.Source]
	if !ok {
		return nil
	}
	events := make([]FlowEvent, 0, len(pairs))
	for _, p := range pairs {
		events = append(events, FlowEvent{
			MsgType:  "flow",
			TenantID: entry.TenantID,
			Source:   p[0],
			Target:   p[1],
		})
	}
	return events
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
	services := []string{"Internet Gateway", "Nginx Load Balancer", "Auth Service", "Payment Service", "Database Cluster"}
	
	// Başlangıç verileri
	uptimeDataMu.Lock()
	for _, s := range services {
		uptimeData[s] = &UptimeStatus{Service: s, Status: "up", Latency: 10, Uptime: 99.9}
	}
	uptimeDataMu.Unlock()

	for {
		time.Sleep(10 * time.Second)
		
		uptimeDataMu.Lock()
		for _, s := range services {
			// Rastgele latency ve nadir down olma durumu
			lat := rand.Intn(40) + 5
			status := "up"
			
			// %5 şansla degraded, %1 şansla down
			roll := rand.Intn(100)
			if roll < 1 {
				status = "down"
				lat = 0
			} else if roll < 6 {
				status = "degraded"
				lat += 200
			}
			
			uptimeData[s].Latency = lat
			uptimeData[s].Status = status
		}
		uptimeDataMu.Unlock()
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
	// AUTHENTICATED ENDPOINTS (API Key zorunlu)
	// -------------------------------------------------------
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
		c.JSON(http.StatusOK, currentTopology)
	})

	// Topolojiyi güncelleme endpoint'i
	api.POST("/topology", func(c *gin.Context) {
		var req TopologyData
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		
		topologyMu.Lock()
		currentTopology = req
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

		// Worker pool'a gönder (non-blocking)
		select {
		case logQueue <- entry:
			c.JSON(http.StatusAccepted, gin.H{"status": "queued"})
		default:
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "Kuyruk dolu, lütfen bekleyin"})
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

	fmt.Println("🚀 SecureStream hazır → http://localhost:8080")
	r.Run(":8080")
}
