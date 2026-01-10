package main

import (
	"fmt"
	"net/http"
	"sync"        // Senkronizasyon işlemleri (WaitGroup, Mutex) için
	"sync/atomic" // Güvenli sayaç işlemleri için (Yarış durumu oluşmaması için)
	"time"        // Zamanlatma ve uyutma işlemleri için

	"github.com/gin-gonic/gin"     // Web Framework (HTTP sunucusu)
	"github.com/gorilla/websocket" // WebSocket protokolü kütüphanesi
)

// ---------------------------------------------------------
// VERİ YAPILARI (STRUCTS)
// ---------------------------------------------------------

// Task: İşlenecek görevin şablonu.
// `json:"id"` etiketleri, bu yapı JSON'a çevrilirken hangi ismi alacağını belirler.
type Task struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// TaskResult: İş bittikten sonra Frontend'e (React) göndereceğimiz sonuç paketi.
type TaskResult struct {
	ID      int    `json:"id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// ---------------------------------------------------------
// GLOBAL DEĞİŞKENLER VE KANALLAR
// ---------------------------------------------------------

// taskQueue: Gelen işlerin sıraya girdiği kanal (Conveyor Belt).
// Kapasitesi 10'dur. Dolarsa gönderen bekler (bloklanır).
var taskQueue = make(chan Task, 10)

// doneReports: Biten işlerin raporlandığı kanal.
// Worker buraya yazar, WebSocket yayıncısı buradan okur.
var doneReports = make(chan TaskResult, 10)

// upgrader: HTTP bağlantısını WebSocket bağlantısına yükselten araç.
// Normalde tarayıcılar "CheckOrigin" ile güvenlik kontrolü yapar.
// Biz geliştirme ortamında olduğumuz için herkese (true) izin veriyoruz.
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// clients: O an sisteme bağlı olan tüm WebSocket kullanıcılarının listesi.
// Key: Bağlantı objesi (*websocket.Conn), Value: true (aktif)
var clients = make(map[*websocket.Conn]bool)

// clientsMu: Mutex (Mutual Exclusion) kilidi.
// `clients` haritasına aynı anda hem yazıp hem okumayı engeller.
// Go'da map'ler "thread-safe" değildir, bu kilit şarttır.
var clientsMu sync.Mutex

// ---------------------------------------------------------
// WORKER (İŞÇİ) FONKSİYONU
// ---------------------------------------------------------
// Bu fonksiyon bir Goroutine olarak çalışır ve asla durmaz (kanal kapanana kadar).
func worker(id int, tasks <-chan Task, results chan<- TaskResult, wg *sync.WaitGroup) {
	// tasks kanalından veri geldiği sürece döngü döner.
	for t := range tasks {
		fmt.Printf("👷 Worker %d: %s (ID: %d) işine başladı...\n", id, t.Name, t.ID)

		// 3 Saniye bekle (Ağır bir veritabanı işlemi veya dosya yükleme simülasyonu)
		time.Sleep(3 * time.Second)

		// İş bitti! Sonucu hazırla.
		result := TaskResult{
			ID:      t.ID,
			Status:  "Tamamlandı ✅",
			Message: fmt.Sprintf("Worker %d tarafından işlendi", id),
		}

		// Hazırlanan sonucu çıkış kanalına fırlat
		results <- result
	}
}

// ---------------------------------------------------------
// ANA FONKSİYON (MAIN)
// ---------------------------------------------------------
func main() {
	r := gin.Default()    // Gin motorunu başlat
	var wg sync.WaitGroup // İşçileri takip etmek için sayaç (burada aktif kullanmıyoruz ama yapı gereği duruyor)

	// CORS MIDDLEWARE
	// React (farklı portta) ile Go (farklı portta) konuşabilsin diye izin veriyoruz.
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Next()
	})

	// 1. ADIM: WORKER POOL BAŞLATMA
	// 3 tane işçiyi işe alıyoruz ve arka plana (goroutine) gönderiyoruz.
	for i := 1; i <= 3; i++ {
		wg.Add(1)
		go worker(i, taskQueue, doneReports, &wg)
	}

	// 2. ADIM: BROADCASTER BAŞLATMA
	// Biten işleri WebSocket'ten dağıtacak olan fonksiyonu arka planda başlatıyoruz.
	go handleMessages()

	// 3. ADIM: WEBSOCKET ENDPOINT (/ws)
	// React buraya bağlanarak canlı veri akışını dinlemeye başlar.
	r.GET("/ws", func(c *gin.Context) {
		// HTTP isteğini WebSocket bağlantısına dönüştür (Upgrade)
		ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			fmt.Println("WS Hatası:", err)
			return
		}

		// Yeni gelen bağlantıyı listeye ekle.
		// DİKKAT: Listeye yazarken kapıyı kilitliyoruz (Lock).
		clientsMu.Lock()
		clients[ws] = true
		clientsMu.Unlock() // İşimiz bitince kilidi açıyoruz.

		fmt.Println("🔌 Yeni bir istemci bağlandı!")
	})

	// 4. ADIM: GÖREV EKLEME ENDPOINT (/add-task)
	// Kullanıcı butona bastığında burası çalışır.
	var taskIDCounter int64 = 0 // Sayaç
	r.GET("/add-task", func(c *gin.Context) {
		name := c.DefaultQuery("name", "Bilinmeyen Görev")

		// Atomik artırma: Aynı anda 1000 istek gelse bile ID'ler karışmaz.
		newID := atomic.AddInt64(&taskIDCounter, 1)

		newTask := Task{ID: int(newID), Name: name}

		// Görevi kanala atıyoruz.
		// Worker'lar hemen buradan kapacak.
		taskQueue <- newTask

		// Kullanıcıya "Tamam aldım" diyoruz (İş henüz bitmedi!)
		c.JSON(http.StatusAccepted, gin.H{"task": newTask, "status": "Queued"})
	})

	fmt.Println("🚀 Sistem WebSocket destekli olarak hazır :8080")

	// Sunucuyu başlat ve sonsuz döngüde dinle
	r.Run(":8080")
}

// ---------------------------------------------------------
// BROADCASTER (YAYINCI) FONKSİYONU
// ---------------------------------------------------------
// Bu fonksiyon doneReports kanalını dinler ve gelen veriyi herkese dağıtır.
func handleMessages() {
	// doneReports kanalına bir veri düştüğü an bu döngü çalışır.
	for result := range doneReports {
		fmt.Printf("📢 Rapor Dağıtılıyor: ID %d bitti.\n", result.ID)

		// Listeyi okuyacağımız için yine kilitliyoruz.
		clientsMu.Lock()
		for client := range clients {
			// Her bir bağlı kullanıcıya JSON verisini gönderiyoruz.
			err := client.WriteJSON(result)

			// Eğer gönderirken hata alırsak (kullanıcı sayfayı kapatmış olabilir)
			if err != nil {
				fmt.Println("Bir kullanıcı düştü.")
				client.Close()          // Bağlantıyı tamamen kapat
				delete(clients, client) // Listeden sil
			}
		}
		clientsMu.Unlock() // Kilidi aç
	}
}
