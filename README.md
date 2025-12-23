 # Concurrent Log Streamer

Bu proje, Go dilinin ileri seviye özelliklerini (Go Routines, Channels, WebSockets) kullanarak yüksek performanslı bir veri işleme ve anlık izleme sisteminin nasıl kurulacağını göstermek amacıyla geliştirilmiştir.

##  Öne Çıkan Özellikler
- **Backend (Go):** - Eşzamanlı (Concurrent) veri üretimi ve işlenmesi.
    - Go Routines ile worker pool mimarisi.
    - Tip güvenli kanal (Channel) yönetimi.
    - WebSocket üzerinden gerçek zamanlı veri akışı.
- **Frontend (React):** - Backend'den gelen anlık verilerin reaktif olarak görselleştirilmesi.

##  Teknolojiler
- **Dil:** Go (Golang)
- **Concurrency:** Goroutines, Channels, Select, Mutex
- **API/Communication:** Gorilla WebSocket, Standard Net/HTTP
- **Frontend:** React, Vite, TailwindCSS

##  Amaç
Sistemin amacı, arka planda yoğun bir şekilde üretilen log ve metrik verilerini ana thread'i bloklamadan işlemek ve en düşük gecikmeyle kullanıcı arayüzüne yansıtmaktır.
