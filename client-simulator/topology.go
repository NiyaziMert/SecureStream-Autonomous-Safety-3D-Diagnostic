package main

func buildTopology() TopologyData {
	nodes := []TopologyNode{
		// ── Layer 0: Network Edge ──
		{ID: "internet", Name: "Internet Traffic", Val: 14, Group: 1, Color: "#94a3b8", NodeType: "edge", Tech: "Global CDN", Description: "Inbound user traffic from worldwide endpoints"},
		{ID: "cdn", Name: "CDN Edge", Val: 8, Group: 1, Color: "#a78bfa", NodeType: "edge", Tech: "CloudFront", Description: "Static asset delivery and DDoS shield"},
		{ID: "dns", Name: "DNS Resolver", Val: 6, Group: 1, Color: "#c084fc", NodeType: "edge", Tech: "Route53", Description: "Domain resolution and geo-routing"},
		{ID: "waf", Name: "Cloudflare WAF", Val: 9, Group: 2, Color: "#f59e0b", NodeType: "security", Tech: "Cloudflare Enterprise", Description: "Web Application Firewall — L7 filtering, rate limiting, bot detection"},
		{ID: "nginx", Name: "Nginx Load Balancer", Val: 8, Group: 3, Color: "#22c55e", NodeType: "gateway", Tech: "Nginx 1.25 + OpenResty", Description: "Reverse proxy, SSL termination, request routing to backend services"},

		// ── Layer 1: Core Services ──
		{ID: "api_gateway", Name: "API Gateway", Val: 7, Group: 3, Color: "#10b981", NodeType: "gateway", Tech: "Kong Gateway", Description: "API versioning, request transformation, auth forwarding"},
		{ID: "auth_service", Name: "Auth Service", Val: 7, Group: 4, Color: "#0ea5e9", NodeType: "service", Tech: "Go + gRPC", Description: "Authentication, authorization, token management"},
		{ID: "user_service", Name: "User Service", Val: 6, Group: 4, Color: "#0ea5e9", NodeType: "service", Tech: "Go + REST", Description: "User profiles, preferences, account management"},
		{ID: "product_service", Name: "Product Service", Val: 7, Group: 4, Color: "#0ea5e9", NodeType: "service", Tech: "Go + GraphQL", Description: "Product catalog, pricing, categories"},
		{ID: "order_service", Name: "Order Service", Val: 7, Group: 4, Color: "#0ea5e9", NodeType: "service", Tech: "Go + gRPC", Description: "Order lifecycle, cart management, checkout flow"},
		{ID: "payment_service", Name: "Payment Service", Val: 8, Group: 4, Color: "#0ea5e9", NodeType: "service", Tech: "Go + PCI-DSS", Description: "Payment processing, refunds, subscription billing"},
		{ID: "notification_service", Name: "Notification Svc", Val: 5, Group: 4, Color: "#0ea5e9", NodeType: "service", Tech: "Go + SMTP/FCM", Description: "Email, SMS, push notifications via templates"},
		{ID: "analytics_service", Name: "Analytics Engine", Val: 6, Group: 4, Color: "#06b6d4", NodeType: "service", Tech: "Python + Pandas", Description: "Real-time metrics, user behavior analysis, funnel tracking"},
		{ID: "logging_service", Name: "Log Aggregator", Val: 5, Group: 4, Color: "#06b6d4", NodeType: "service", Tech: "Fluentd + Go", Description: "Centralized log collection, parsing, and forwarding"},

		// ── Layer 1: Data Stores ──
		{ID: "db_master", Name: "PostgreSQL Master", Val: 9, Group: 5, Color: "#8b5cf6", NodeType: "database", Tech: "PostgreSQL 16", Description: "Primary OLTP database — writes, transactions, schema migrations"},
		{ID: "db_replica", Name: "PostgreSQL Replica", Val: 7, Group: 5, Color: "#7c3aed", NodeType: "database", Tech: "PostgreSQL 16 (streaming)", Description: "Read replica for query load distribution"},
		{ID: "redis", Name: "Redis Cache", Val: 7, Group: 6, Color: "#ef4444", NodeType: "cache", Tech: "Redis 7 Cluster", Description: "Session store, API cache, rate limit counters, pub/sub"},
		{ID: "elasticsearch", Name: "Elasticsearch", Val: 6, Group: 6, Color: "#f97316", NodeType: "database", Tech: "ES 8.x + Kibana", Description: "Full-text search, log indexing, analytics queries"},
		{ID: "kafka", Name: "Kafka Broker", Val: 7, Group: 6, Color: "#22c55e", NodeType: "queue", Tech: "Apache Kafka 3.7", Description: "Event streaming — order events, audit logs, CDC"},
		{ID: "s3_storage", Name: "Object Storage", Val: 5, Group: 6, Color: "#64748b", NodeType: "storage", Tech: "MinIO / S3", Description: "Product images, invoices, log archives"},

		// ── Layer 2: Auth Microservices ──
		{ID: "auth_jwt", Name: "JWT Service", Val: 4, Group: 4, Color: "#38bdf8", Parent: "auth_service", NodeType: "function", Tech: "Go", Description: "JWT token generation, validation, refresh rotation"},
		{ID: "auth_oauth", Name: "OAuth2 Provider", Val: 4, Group: 4, Color: "#38bdf8", Parent: "auth_service", NodeType: "function", Tech: "Go + OAuth2", Description: "Google/GitHub/Apple SSO integration"},
		{ID: "auth_session", Name: "Session Manager", Val: 3, Group: 4, Color: "#38bdf8", Parent: "auth_service", NodeType: "function", Tech: "Go + Redis", Description: "Session lifecycle, TTL management, concurrent login control"},
		{ID: "auth_mfa", Name: "MFA Service", Val: 3, Group: 4, Color: "#38bdf8", Parent: "auth_service", NodeType: "function", Tech: "Go + TOTP", Description: "Two-factor authentication via TOTP and SMS"},

		// ── Layer 2: Product Microservices ──
		{ID: "prod_search", Name: "Search Engine", Val: 4, Group: 4, Color: "#34d399", Parent: "product_service", NodeType: "function", Tech: "Go + ES", Description: "Full-text product search with filters and facets"},
		{ID: "prod_inventory", Name: "Inventory Manager", Val: 4, Group: 4, Color: "#34d399", Parent: "product_service", NodeType: "function", Tech: "Go", Description: "Stock levels, warehouse allocation, low-stock alerts"},
		{ID: "prod_recommend", Name: "Recommendation AI", Val: 4, Group: 4, Color: "#34d399", Parent: "product_service", NodeType: "function", Tech: "Python + ML", Description: "Collaborative filtering, similar products, personalization"},
		{ID: "prod_pricing", Name: "Dynamic Pricing", Val: 3, Group: 4, Color: "#34d399", Parent: "product_service", NodeType: "function", Tech: "Go", Description: "Real-time price calculation, discounts, A/B pricing"},

		// ── Layer 2: Payment Microservices ──
		{ID: "pay_stripe", Name: "Stripe Gateway", Val: 5, Group: 4, Color: "#818cf8", Parent: "payment_service", NodeType: "function", Tech: "Go + Stripe SDK", Description: "Credit card processing via Stripe API"},
		{ID: "pay_paypal", Name: "PayPal Gateway", Val: 4, Group: 4, Color: "#818cf8", Parent: "payment_service", NodeType: "function", Tech: "Go + PayPal SDK", Description: "PayPal checkout integration"},
		{ID: "pay_fraud", Name: "Fraud Detection AI", Val: 5, Group: 4, Color: "#818cf8", Parent: "payment_service", NodeType: "function", Tech: "Python + TensorFlow", Description: "ML-based fraud scoring, velocity checks, device fingerprinting"},
		{ID: "pay_invoice", Name: "Invoice Service", Val: 3, Group: 4, Color: "#818cf8", Parent: "payment_service", NodeType: "function", Tech: "Go + PDF", Description: "Invoice generation, tax calculation, PDF rendering"},

		// ── Layer 2: Order Microservices ──
		{ID: "order_cart", Name: "Cart Manager", Val: 4, Group: 4, Color: "#fbbf24", Parent: "order_service", NodeType: "function", Tech: "Go + Redis", Description: "Shopping cart CRUD, item validation, price sync"},
		{ID: "order_checkout", Name: "Checkout Flow", Val: 4, Group: 4, Color: "#fbbf24", Parent: "order_service", NodeType: "function", Tech: "Go + Saga", Description: "Distributed checkout saga — inventory reserve, payment, confirm"},
		{ID: "order_tracking", Name: "Order Tracker", Val: 3, Group: 4, Color: "#fbbf24", Parent: "order_service", NodeType: "function", Tech: "Go + Kafka", Description: "Order status updates, shipping integration, ETA calculation"},

		// ── Layer 3: Stripe Functions ──
		{ID: "fn_stripe_validate", Name: "ValidateToken()", Val: 2, Group: 4, Color: "#c4b5fd", Parent: "pay_stripe", NodeType: "function", Tech: "Go", Description: "Card token validation and 3DS check"},
		{ID: "fn_stripe_charge", Name: "ProcessCharge()", Val: 2, Group: 4, Color: "#c4b5fd", Parent: "pay_stripe", NodeType: "function", Tech: "Go", Description: "Execute payment charge via Stripe PaymentIntent API"},
		{ID: "fn_stripe_refund", Name: "RefundFlow()", Val: 2, Group: 4, Color: "#c4b5fd", Parent: "pay_stripe", NodeType: "function", Tech: "Go", Description: "Process full/partial refunds back to customer"},
		{ID: "fn_stripe_webhook", Name: "WebhookHandler()", Val: 2, Group: 4, Color: "#c4b5fd", Parent: "pay_stripe", NodeType: "function", Tech: "Go", Description: "Handle async Stripe webhook events (success, failure, dispute)"},
	}

	links := []TopologyLink{
		// Edge chain
		{Source: "internet", Target: "dns", Val: 8, Label: "DNS lookup"},
		{Source: "internet", Target: "cdn", Val: 7, Label: "Static assets"},
		{Source: "internet", Target: "waf", Val: 10, Label: "HTTPS requests"},
		{Source: "dns", Target: "waf", Val: 6, Label: "Resolved IP"},
		{Source: "cdn", Target: "s3_storage", Val: 4, Label: "Cache miss fetch"},
		{Source: "waf", Target: "nginx", Val: 9, Label: "Filtered traffic"},
		{Source: "nginx", Target: "api_gateway", Val: 8, Label: "Proxy pass /api/*"},

		// API Gateway → Services
		{Source: "api_gateway", Target: "auth_service", Val: 6, Label: "POST /auth/login"},
		{Source: "api_gateway", Target: "user_service", Val: 4, Label: "GET /users/:id"},
		{Source: "api_gateway", Target: "product_service", Val: 7, Label: "GET /products"},
		{Source: "api_gateway", Target: "order_service", Val: 5, Label: "POST /orders"},
		{Source: "api_gateway", Target: "payment_service", Val: 5, Label: "POST /payments"},
		{Source: "api_gateway", Target: "analytics_service", Val: 3, Label: "Event tracking"},

		// Service → Data stores
		{Source: "auth_service", Target: "db_master", Val: 4, Label: "SELECT users WHERE credentials"},
		{Source: "auth_service", Target: "redis", Val: 5, Label: "Session token cache"},
		{Source: "user_service", Target: "db_master", Val: 3, Label: "User CRUD queries"},
		{Source: "user_service", Target: "redis", Val: 3, Label: "Profile cache"},
		{Source: "product_service", Target: "redis", Val: 5, Label: "Product cache GET/SET"},
		{Source: "product_service", Target: "db_replica", Val: 4, Label: "SELECT products JOIN categories"},
		{Source: "product_service", Target: "elasticsearch", Val: 3, Label: "Search index query"},
		{Source: "order_service", Target: "db_master", Val: 5, Label: "INSERT orders, UPDATE stock"},
		{Source: "order_service", Target: "redis", Val: 3, Label: "Cart session data"},
		{Source: "order_service", Target: "kafka", Val: 4, Label: "order.created event"},
		{Source: "payment_service", Target: "db_master", Val: 4, Label: "INSERT transactions"},
		{Source: "payment_service", Target: "kafka", Val: 3, Label: "payment.completed event"},
		{Source: "notification_service", Target: "redis", Val: 2, Label: "Template cache"},
		{Source: "notification_service", Target: "kafka", Val: 3, Label: "Consume notification events"},
		{Source: "analytics_service", Target: "elasticsearch", Val: 4, Label: "Aggregation queries"},
		{Source: "analytics_service", Target: "kafka", Val: 3, Label: "Consume analytics events"},
		{Source: "logging_service", Target: "elasticsearch", Val: 5, Label: "Bulk index logs"},
		{Source: "logging_service", Target: "kafka", Val: 4, Label: "Consume log stream"},
		{Source: "logging_service", Target: "s3_storage", Val: 2, Label: "Archive old logs"},

		// Inter-service
		{Source: "order_service", Target: "payment_service", Val: 4, Label: "gRPC: ChargeOrder()"},
		{Source: "order_service", Target: "notification_service", Val: 3, Label: "gRPC: SendConfirmation()"},
		{Source: "order_service", Target: "product_service", Val: 3, Label: "gRPC: ReserveStock()"},

		// Auth microservices
		{Source: "auth_service", Target: "auth_jwt", Val: 3, Label: "GenerateJWT()"},
		{Source: "auth_service", Target: "auth_oauth", Val: 2, Label: "OAuth2 flow"},
		{Source: "auth_service", Target: "auth_session", Val: 3, Label: "CreateSession()"},
		{Source: "auth_service", Target: "auth_mfa", Val: 2, Label: "VerifyTOTP()"},
		{Source: "auth_jwt", Target: "redis", Val: 3, Label: "Token blacklist check"},
		{Source: "auth_oauth", Target: "db_master", Val: 2, Label: "Upsert OAuth user"},
		{Source: "auth_session", Target: "redis", Val: 3, Label: "SET session TTL"},
		{Source: "auth_mfa", Target: "redis", Val: 2, Label: "TOTP rate limit"},

		// Product microservices
		{Source: "product_service", Target: "prod_search", Val: 3, Label: "FullTextSearch()"},
		{Source: "product_service", Target: "prod_inventory", Val: 3, Label: "CheckStock()"},
		{Source: "product_service", Target: "prod_recommend", Val: 2, Label: "GetRecommendations()"},
		{Source: "product_service", Target: "prod_pricing", Val: 2, Label: "CalcPrice()"},
		{Source: "prod_search", Target: "elasticsearch", Val: 3, Label: "ES multi_match query"},
		{Source: "prod_inventory", Target: "db_replica", Val: 3, Label: "SELECT stock WHERE sku"},
		{Source: "prod_recommend", Target: "redis", Val: 2, Label: "ML model cache"},
		{Source: "prod_pricing", Target: "redis", Val: 2, Label: "Price rule cache"},

		// Payment microservices
		{Source: "payment_service", Target: "pay_stripe", Val: 3, Label: "ProcessStripePayment()"},
		{Source: "payment_service", Target: "pay_paypal", Val: 2, Label: "ProcessPayPalPayment()"},
		{Source: "payment_service", Target: "pay_fraud", Val: 4, Label: "ScoreTransaction()"},
		{Source: "payment_service", Target: "pay_invoice", Val: 2, Label: "GenerateInvoice()"},
		{Source: "pay_stripe", Target: "db_master", Val: 2, Label: "Log Stripe txn"},
		{Source: "pay_paypal", Target: "db_master", Val: 2, Label: "Log PayPal txn"},
		{Source: "pay_fraud", Target: "db_master", Val: 3, Label: "Fraud history query"},
		{Source: "pay_invoice", Target: "db_master", Val: 2, Label: "Invoice record"},
		{Source: "pay_invoice", Target: "s3_storage", Val: 2, Label: "Upload PDF"},
		{Source: "pay_stripe", Target: "pay_fraud", Val: 3, Label: "Pre-charge fraud check"},
		{Source: "pay_paypal", Target: "pay_fraud", Val: 2, Label: "Pre-charge fraud check"},

		// Order microservices
		{Source: "order_service", Target: "order_cart", Val: 3, Label: "ManageCart()"},
		{Source: "order_service", Target: "order_checkout", Val: 4, Label: "InitCheckout()"},
		{Source: "order_service", Target: "order_tracking", Val: 2, Label: "TrackOrder()"},
		{Source: "order_cart", Target: "redis", Val: 3, Label: "Cart items HASH"},
		{Source: "order_checkout", Target: "payment_service", Val: 4, Label: "ChargeOrder()"},
		{Source: "order_checkout", Target: "db_master", Val: 3, Label: "Commit order txn"},
		{Source: "order_tracking", Target: "kafka", Val: 2, Label: "order.status.updated"},

		// Stripe function chain
		{Source: "pay_stripe", Target: "fn_stripe_validate", Val: 3, Label: "Validate card token"},
		{Source: "fn_stripe_validate", Target: "fn_stripe_charge", Val: 3, Label: "Token OK → charge"},
		{Source: "fn_stripe_validate", Target: "fn_stripe_refund", Val: 1, Label: "Refund request"},
		{Source: "fn_stripe_charge", Target: "fn_stripe_webhook", Val: 2, Label: "Async event"},
		{Source: "fn_stripe_charge", Target: "db_master", Val: 2, Label: "Record charge"},
		{Source: "fn_stripe_refund", Target: "db_master", Val: 1, Label: "Record refund"},

		// DB replication
		{Source: "db_master", Target: "db_replica", Val: 4, Label: "WAL streaming replication"},
	}

	return TopologyData{Nodes: nodes, Links: links}
}
