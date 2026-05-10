package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"
)

const (
	apiKey     = "dev-api-key-12345"
	baseURL    = "http://localhost:8080/api"
	tenantName = "Acme E-Commerce Ltd."
)

// ---- Veri Yapıları ----

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

type IngestRequest struct {
	Source string `json:"source"`
	RawLog string `json:"raw_log"`
}

// ---- Fake Kullanıcı Veri Havuzu ----

var firstNames = []string{
	"Ahmet", "Mehmet", "Ayşe", "Fatma", "Ali", "Kemal", "Selin", "Canan",
	"James", "Emily", "Carlos", "Maria", "Lucas", "Sara", "Noah", "Liam",
	"yuki", "hiroshi", "priya", "raj", "chen", "wang", "amina", "kwame",
}
var lastNames = []string{
	"Yılmaz", "Kaya", "Demir", "Smith", "Johnson", "Garcia", "Tanaka",
	"Müller", "Dubois", "Rossi", "Santos", "Kim", "Osei", "Patel",
}

var sessionIDs []string

func init() {
	rand.Seed(time.Now().UnixNano())
	for i := 0; i < 200; i++ {
		sessionIDs = append(sessionIDs, fmt.Sprintf("sess_%08x", rand.Int31()))
	}
}

func randomUser() string {
	return firstNames[rand.Intn(len(firstNames))] + "_" + lastNames[rand.Intn(len(lastNames))]
}

func randomIP() string {
	return fmt.Sprintf("%d.%d.%d.%d",
		rand.Intn(220)+10, rand.Intn(255), rand.Intn(255), rand.Intn(255))
}

func randomSession() string {
	return sessionIDs[rand.Intn(len(sessionIDs))]
}

// ---- Topoloji Tanımı ----

func buildTopology() TopologyData {
	nodes := []TopologyNode{
		// ── Katman 0: Ağ Giriş Noktaları ──
		{ID: "internet", Name: "Internet Traffic", Val: 12, Group: 1, Color: "#94a3b8"},
		{ID: "waf", Name: "Cloudflare WAF", Val: 9, Group: 2, Color: "#f59e0b"},
		{ID: "nginx", Name: "Nginx Load Balancer", Val: 7, Group: 3, Color: "#22c55e"},

		// ── Katman 1: Ana Servisler ──
		{ID: "auth_service", Name: "Auth Service", Val: 6, Group: 4, Color: "#0ea5e9"},
		{ID: "product_service", Name: "Product Service", Val: 6, Group: 4, Color: "#0ea5e9"},
		{ID: "payment_service", Name: "Payment Service", Val: 7, Group: 4, Color: "#0ea5e9"},
		{ID: "notification_service", Name: "Notification Svc", Val: 4, Group: 4, Color: "#0ea5e9"},

		// ── Katman 1: Veri Katmanı ──
		{ID: "db_master", Name: "PostgreSQL Master", Val: 8, Group: 5, Color: "#8b5cf6"},
		{ID: "db_replica", Name: "PostgreSQL Replica", Val: 6, Group: 5, Color: "#7c3aed"},
		{ID: "redis", Name: "Redis Cache", Val: 6, Group: 6, Color: "#ef4444"},
		{ID: "elasticsearch", Name: "Elasticsearch", Val: 5, Group: 6, Color: "#f97316"},

		// ── Katman 2: Auth Service Microservisleri ──
		{ID: "auth_jwt", Name: "JWT Service", Val: 4, Group: 4, Color: "#38bdf8", Parent: "auth_service"},
		{ID: "auth_oauth", Name: "OAuth2 Provider", Val: 4, Group: 4, Color: "#38bdf8", Parent: "auth_service"},
		{ID: "auth_session", Name: "Session Manager", Val: 3, Group: 4, Color: "#38bdf8", Parent: "auth_service"},

		// ── Katman 2: Product Service Microservisleri ──
		{ID: "prod_search", Name: "Search Engine", Val: 4, Group: 4, Color: "#34d399", Parent: "product_service"},
		{ID: "prod_inventory", Name: "Inventory Manager", Val: 4, Group: 4, Color: "#34d399", Parent: "product_service"},
		{ID: "prod_recommend", Name: "Recommendation AI", Val: 4, Group: 4, Color: "#34d399", Parent: "product_service"},

		// ── Katman 2: Payment Service Microservisleri ──
		{ID: "pay_stripe", Name: "Stripe Gateway", Val: 5, Group: 4, Color: "#818cf8", Parent: "payment_service"},
		{ID: "pay_paypal", Name: "PayPal Gateway", Val: 4, Group: 4, Color: "#818cf8", Parent: "payment_service"},
		{ID: "pay_fraud", Name: "Fraud Detection AI", Val: 5, Group: 4, Color: "#818cf8", Parent: "payment_service"},
		{ID: "pay_invoice", Name: "Invoice Service", Val: 3, Group: 4, Color: "#818cf8", Parent: "payment_service"},

		// ── Katman 3: Stripe Gateway Fonksiyonları ──
		{ID: "fn_stripe_validate", Name: "ValidateToken()", Val: 2, Group: 4, Color: "#c4b5fd", Parent: "pay_stripe"},
		{ID: "fn_stripe_charge", Name: "ProcessCharge()", Val: 2, Group: 4, Color: "#c4b5fd", Parent: "pay_stripe"},
		{ID: "fn_stripe_refund", Name: "RefundFlow()", Val: 2, Group: 4, Color: "#c4b5fd", Parent: "pay_stripe"},
		{ID: "fn_stripe_webhook", Name: "WebhookHandler()", Val: 2, Group: 4, Color: "#c4b5fd", Parent: "pay_stripe"},
	}

	links := []TopologyLink{
		// Giriş zinciri
		{Source: "internet", Target: "waf", Val: 8},
		{Source: "waf", Target: "nginx", Val: 8},

		// Nginx → Servisler
		{Source: "nginx", Target: "auth_service", Val: 5},
		{Source: "nginx", Target: "product_service", Val: 6},
		{Source: "nginx", Target: "payment_service", Val: 4},
		{Source: "nginx", Target: "notification_service", Val: 2},

		// Auth → DB / Redis
		{Source: "auth_service", Target: "db_master", Val: 3},
		{Source: "auth_service", Target: "redis", Val: 4},

		// Auth microservisler
		{Source: "auth_service", Target: "auth_jwt", Val: 3},
		{Source: "auth_service", Target: "auth_oauth", Val: 2},
		{Source: "auth_service", Target: "auth_session", Val: 3},
		{Source: "auth_jwt", Target: "redis", Val: 3},
		{Source: "auth_oauth", Target: "db_master", Val: 2},
		{Source: "auth_session", Target: "redis", Val: 3},

		// Product → DB / Redis / ES
		{Source: "product_service", Target: "redis", Val: 4},
		{Source: "product_service", Target: "db_replica", Val: 3},
		{Source: "product_service", Target: "elasticsearch", Val: 2},

		// Product microservisler
		{Source: "product_service", Target: "prod_search", Val: 3},
		{Source: "product_service", Target: "prod_inventory", Val: 3},
		{Source: "product_service", Target: "prod_recommend", Val: 2},
		{Source: "prod_search", Target: "elasticsearch", Val: 3},
		{Source: "prod_inventory", Target: "db_replica", Val: 3},
		{Source: "prod_recommend", Target: "redis", Val: 2},

		// Payment microservisler
		{Source: "payment_service", Target: "pay_stripe", Val: 3},
		{Source: "payment_service", Target: "pay_paypal", Val: 2},
		{Source: "payment_service", Target: "pay_fraud", Val: 4},
		{Source: "payment_service", Target: "pay_invoice", Val: 2},
		{Source: "pay_stripe", Target: "db_master", Val: 2},
		{Source: "pay_paypal", Target: "db_master", Val: 2},
		{Source: "pay_fraud", Target: "db_master", Val: 3},
		{Source: "pay_invoice", Target: "db_master", Val: 2},
		{Source: "pay_stripe", Target: "pay_fraud", Val: 3},
		{Source: "pay_paypal", Target: "pay_fraud", Val: 2},

		// Stripe fonksiyon zinciri
		{Source: "pay_stripe", Target: "fn_stripe_validate", Val: 3},
		{Source: "fn_stripe_validate", Target: "fn_stripe_charge", Val: 3},
		{Source: "fn_stripe_validate", Target: "fn_stripe_refund", Val: 1},
		{Source: "fn_stripe_charge", Target: "fn_stripe_webhook", Val: 2},
		{Source: "fn_stripe_charge", Target: "db_master", Val: 2},
		{Source: "fn_stripe_refund", Target: "db_master", Val: 1},

		// DB replikasyonu
		{Source: "db_master", Target: "db_replica", Val: 3},

		// Notification
		{Source: "notification_service", Target: "redis", Val: 2},
	}

	return TopologyData{Nodes: nodes, Links: links}
}

// ---- Log Üreticileri ----

// normalLogs: Sisteme gerçekçi normal trafik logları üretir
func buildNormalLog() (string, string) {
	roll := rand.Intn(100)
	user := randomUser()
	session := randomSession()
	ip := randomIP()
	productID := rand.Intn(5000) + 1
	orderID := rand.Intn(100000) + 10000

	switch {
	// ── Auth service logs (25%) ──
	case roll < 10:
		return "auth_service", fmt.Sprintf(
			`[INFO] User %s authenticated successfully from %s session=%s`,
			user, ip, session)
	case roll < 15:
		return "auth_jwt", fmt.Sprintf(
			`[DEBUG] JWT issued for user=%s exp=3600s session=%s`, user, session)
	case roll < 18:
		return "auth_oauth", fmt.Sprintf(
			`[INFO] OAuth2 token refresh for user=%s provider=google`, user)
	case roll < 25:
		return "auth_session", fmt.Sprintf(
			`[DEBUG] Session %s extended TTL=1800 user=%s ip=%s`, session, user, ip)

	// ── Product service logs (30%) ──
	case roll < 35:
		return "product_service", fmt.Sprintf(
			`[INFO] GET /api/v1/products?page=%d&limit=20 user=%s 200 %dms`,
			rand.Intn(100), user, rand.Intn(80)+10)
	case roll < 40:
		return "prod_search", fmt.Sprintf(
			`[INFO] Search query="%s" hits=%d user=%s took=%dms`,
			[]string{"sneakers", "laptop", "headphones", "jacket", "watch"}[rand.Intn(5)],
			rand.Intn(200)+1, user, rand.Intn(50)+5)
	case roll < 45:
		return "prod_inventory", fmt.Sprintf(
			`[DEBUG] Stock check product_id=%d qty=%d warehouse=IST-%02d`,
			productID, rand.Intn(500), rand.Intn(5)+1)
	case roll < 55:
		return "prod_recommend", fmt.Sprintf(
			`[INFO] Recommendations generated for user=%s model=collaborative items=%d latency=%dms`,
			user, rand.Intn(10)+5, rand.Intn(120)+20)

	// ── Payment service logs (20%) ──
	case roll < 60:
		return "payment_service", fmt.Sprintf(
			`[INFO] Payment initiated order_id=%d user=%s amount=%.2f currency=USD`,
			orderID, user, float64(rand.Intn(50000)+500)/100.0)
	case roll < 65:
		return "pay_stripe", fmt.Sprintf(
			`[INFO] Stripe charge order_id=%d amount=%.2f status=succeeded`,
			orderID, float64(rand.Intn(50000)+500)/100.0)
	case roll < 68:
		return "fn_stripe_validate", fmt.Sprintf(
			`[DEBUG] ValidateToken() card_brand=%s last4=%04d user=%s ok=true`,
			[]string{"visa", "mastercard", "amex"}[rand.Intn(3)],
			rand.Intn(9000)+1000, user)
	case roll < 73:
		return "fn_stripe_charge", fmt.Sprintf(
			`[INFO] ProcessCharge() order_id=%d amount=%.2f txn_id=pi_%08x ok=true`,
			orderID, float64(rand.Intn(50000)+500)/100.0, rand.Int31())
	case roll < 75:
		return "fn_stripe_webhook", fmt.Sprintf(
			`[INFO] WebhookHandler() event=payment_intent.succeeded order_id=%d`, orderID)
	case roll < 78:
		return "pay_fraud", fmt.Sprintf(
			`[INFO] FraudCheck order_id=%d user=%s score=%.2f action=allow`,
			orderID, user, rand.Float64()*0.3)

	// ── Infrastructure logs (25%) ──
	case roll < 83:
		return "nginx", fmt.Sprintf(
			`%s - %s [%s] "GET /api/v1/products HTTP/1.1" 200 %d "-" "Mozilla/5.0"`,
			ip, user, time.Now().Format("02/Jan/2006:15:04:05 -0700"), rand.Intn(5000)+500)
	case roll < 87:
		return "redis", fmt.Sprintf(
			`[DEBUG] CACHE HIT key=product:%d ttl=%ds`, productID, rand.Intn(300)+60)
	case roll < 91:
		return "db_master", fmt.Sprintf(
			`[LOG] duration: %.3f ms  statement: SELECT * FROM orders WHERE user_id = $1 LIMIT 20`,
			rand.Float64()*10+0.5)
	case roll < 94:
		return "db_replica", fmt.Sprintf(
			`[LOG] duration: %.3f ms  statement: SELECT * FROM products WHERE category_id = $1`,
			rand.Float64()*5+0.2)
	case roll < 97:
		return "waf", fmt.Sprintf(
			`ALLOW IN=eth0 SRC=%s DST=10.0.0.10 PROTO=TCP DPT=443 user=%s`, ip, user)
	default:
		return "notification_service", fmt.Sprintf(
			`[INFO] Email sent to=%s@example.com type=order_confirmation order_id=%d`,
			user, orderID)
	}
}

// attackLogs: Güvenlik saldırı logları üretir
func buildAttackLog() (string, string) {
	ip := randomIP()
	user := randomUser()
	orderID := rand.Intn(100000) + 10000

	switch rand.Intn(7) {
	case 0:
		return "auth_service", fmt.Sprintf(
			`Failed password for root from %s port 22 ssh2`, ip)
	case 1:
		return "auth_service", fmt.Sprintf(
			`Invalid user %s from %s`, user, ip)
	case 2:
		return "db_master", fmt.Sprintf(
			`FATAL: password authentication failed for user "postgres" from %s`, ip)
	case 3:
		return "auth_session", fmt.Sprintf(
			`sudo: user %s NOT in sudoers ; TTY=pts/0 ; PWD=/root ; USER=root`, user)
	case 4:
		return "prod_search", fmt.Sprintf(
			`GET /search?q=1 UNION ALL SELECT username,password FROM users-- from %s`, ip)
	case 5:
		return "waf", fmt.Sprintf(
			`DROP IN=eth0 SRC=%s DST=10.0.0.10 LEN=60 PROTO=TCP DPT=3306 SYN`, ip)
	default:
		return "pay_fraud", fmt.Sprintf(
			`[ALERT] Fraud detected order_id=%d user=%s ip=%s score=0.97 action=block`,
			orderID, user, ip)
	}
}

// ---- HTTP Yardımcıları ----

var httpClient = &http.Client{Timeout: 5 * time.Second}

func postJSON(endpoint string, payload interface{}) int {
	data, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", baseURL+endpoint, bytes.NewBuffer(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", apiKey)
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

// ---- Topoloji Gönderici ----

func sendTopology() {
	topology := buildTopology()
	code := postJSON("/topology", topology)
	if code == 200 {
		fmt.Printf("✅ Topology registered: %d nodes, %d links\n",
			len(topology.Nodes), len(topology.Links))
	} else {
		fmt.Printf("⚠️  Topology registration status: %d\n", code)
	}
}

// ---- Ana Simülasyon ----

func main() {
	fmt.Printf("🚀 [%s] Simulator starting...\n", tenantName)

	// Backend hazır olana kadar bekle
	for i := 0; i < 10; i++ {
		resp, err := httpClient.Get("http://localhost:8080/health")
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			fmt.Println("✅ Backend is up!")
			break
		}
		fmt.Printf("⏳ Waiting for backend... (%d/10)\n", i+1)
		time.Sleep(2 * time.Second)
	}

	// Topolojiyi kaydet
	sendTopology()

	fmt.Println("🌊 Starting high-frequency traffic simulation...")
	fmt.Println("   Normal traffic: ~80%  |  Attack traffic: ~20%")

	ticker := time.NewTicker(5 * time.Minute)
	go func() {
		for range ticker.C {
			sendTopology() // Topolojiyi 5 dakikada bir yenile
		}
	}()

	for {
		var source, rawLog string

		if rand.Float32() < 0.20 {
			source, rawLog = buildAttackLog()
		} else {
			source, rawLog = buildNormalLog()
		}

		code := postJSON("/ingest", IngestRequest{Source: source, RawLog: rawLog})
		if code > 0 && code != 202 && code != 429 {
			fmt.Printf("⚠️  Unexpected status %d [%s]\n", code, source)
		}

		// 40-120ms arası bekleme = saniyede ~8-25 log
		time.Sleep(time.Duration(rand.Intn(80)+40) * time.Millisecond)
	}
}
