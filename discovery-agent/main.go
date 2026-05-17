package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const apiKey = "dev-api-key-12345"

type TopologyNode struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Val         int    `json:"val"`
	Group       int    `json:"group"`
	Color       string `json:"color"`
	NodeType    string `json:"node_type,omitempty"`
	Tech        string `json:"tech,omitempty"`
	Description string `json:"description,omitempty"`
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

var (
	// Microservice HTTP çağrılarını yakalamak için (Örn: http://payment_service/api)
	urlRegex = regexp.MustCompile(`https?://([a-zA-Z0-9_-]+)(:\d+)?/`)
	// Docker-compose'daki servis tanımlarını yakalamak için
	dockerRegex = regexp.MustCompile(`^\s*([a-zA-Z0-9_-]+):\s*$`)
)

func main() {
	targetDir := flag.String("dir", ".", "Tarama yapilacak ana dizin")
	backendURL := flag.String("backend", "http://localhost:8080/api", "SecureStream API URL")
	flag.Parse()

	fmt.Printf("🔍 Kodu tarıyor: %s\n", *targetDir)

	nodesMap := make(map[string]TopologyNode)
	linksMap := make(map[string]TopologyLink) // key: source->target

	// Başlangıç için internet node'unu ekleyelim
	nodesMap["internet"] = TopologyNode{ID: "internet", Name: "Internet (External)", Val: 10, Group: 1, Color: "#94a3b8", NodeType: "edge"}

	err := filepath.WalkDir(*targetDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules" || d.Name() == "dist" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(path)
		name := d.Name()
		if ext == ".go" || ext == ".js" || ext == ".py" || name == "docker-compose.yml" || name == "docker-compose.yaml" {
			analyzeFile(path, nodesMap, linksMap)
		}
		return nil
	})

	if err != nil {
		fmt.Printf("Tarama hatası: %v\n", err)
	}

	var topology TopologyData
	for _, n := range nodesMap {
		topology.Nodes = append(topology.Nodes, n)
	}
	for _, l := range linksMap {
		topology.Links = append(topology.Links, l)
	}

	fmt.Printf("✅ Kodu tarama işlemi bitti! Keşfedilen Düğümler: %d, Bağlantılar: %d\n", len(topology.Nodes), len(topology.Links))

	sendTopology(*backendURL, topology)
}

func analyzeFile(path string, nodes map[string]TopologyNode, links map[string]TopologyLink) {
	content, err := os.ReadFile(path)
	if err != nil {
		return
	}
	lines := strings.Split(string(content), "\n")

	// Dosyanın bulunduğu klasörün adını servis adı olarak kabul ediyoruz (Örn: /backend -> backend servisi)
	dirName := filepath.Base(filepath.Dir(path))
	if dirName == "." || dirName == "src" {
		dirName = "core_service"
	}

	// docker-compose dosyasından statik containerları tarama
	if strings.Contains(path, "docker-compose") {
		inServices := false
		for _, line := range lines {
			if strings.HasPrefix(line, "services:") {
				inServices = true
				continue
			}
			if inServices && !strings.HasPrefix(line, " ") { // "volumes:" veya "networks:" geldiğinde çık
				if !strings.HasPrefix(line, "services:") && len(strings.TrimSpace(line)) > 0 {
					inServices = false
				}
			}
			if inServices && dockerRegex.MatchString(line) {
				match := dockerRegex.FindStringSubmatch(line)
				if len(match) > 1 {
					srv := match[1]
					nodes[srv] = TopologyNode{ID: srv, Name: srv + " (Container)", Val: 6, Group: 3, Color: "#0ea5e9", NodeType: "service", Tech: "Docker"}
				}
			}
		}
		return
	}

	// Bu klasörü node olarak ekle
	if _, exists := nodes[dirName]; !exists {
		color := "#3b82f6" // Mavi (Default)
		if strings.Contains(dirName, "db") || strings.Contains(dirName, "postgres") {
			color = "#8b5cf6"
		} else if strings.Contains(dirName, "front") {
			color = "#ec4899"
		}
		nodes[dirName] = TopologyNode{ID: dirName, Name: dirName, Val: 5, Group: 4, Color: color, NodeType: "service"}
		
		// İnternetten bu servise bağlantı varmış gibi varsay (Eğer frontend veya api gateway ise)
		if strings.Contains(dirName, "front") || strings.Contains(dirName, "gateway") {
			links["internet->"+dirName] = TopologyLink{Source: "internet", Target: dirName, Val: 3, Label: "Ingress"}
		}
	}

	for _, line := range lines {
		// HTTP çağrılarını yakala
		matches := urlRegex.FindAllStringSubmatch(line, -1)
		for _, match := range matches {
			if len(match) > 1 {
				target := match[1]
				// Localhost veya bilindik external adresleri yoksay
				if target == "localhost" || target == "127.0.0.1" || strings.Contains(target, "google") {
					continue
				}

				if _, exists := nodes[target]; !exists {
					nodes[target] = TopologyNode{ID: target, Name: target, Val: 4, Group: 5, Color: "#10b981", NodeType: "dependency"}
				}

				linkKey := dirName + "->" + target
				links[linkKey] = TopologyLink{Source: dirName, Target: target, Val: 3, Label: "HTTP API Call"}
			}
		}

		// Redis bağlantısını yakala
		if strings.Contains(line, "redis.NewClient") || strings.Contains(line, "redis:") {
			if _, ok := nodes["redis"]; !ok {
				nodes["redis"] = TopologyNode{ID: "redis", Name: "Redis Cache", Val: 5, Group: 6, Color: "#ef4444", NodeType: "cache"}
			}
			links[dirName+"->redis"] = TopologyLink{Source: dirName, Target: "redis", Val: 4, Label: "Cache Query"}
		}

		// Postgres/SQL bağlantısını yakala
		if strings.Contains(line, "sql.Open(\"postgres\"") || strings.Contains(line, "postgres:") {
			if _, ok := nodes["postgres"]; !ok {
				nodes["postgres"] = TopologyNode{ID: "postgres", Name: "PostgreSQL", Val: 7, Group: 6, Color: "#8b5cf6", NodeType: "database"}
			}
			links[dirName+"->postgres"] = TopologyLink{Source: dirName, Target: "postgres", Val: 5, Label: "SQL Query"}
		}
	}
}

func sendTopology(url string, data TopologyData) {
	fmt.Println("🚀 Çıkarılan topoloji haritası SecureStream Backend'ine pushlanıyor...")
	payload, _ := json.Marshal(data)
	req, _ := http.NewRequest("POST", url+"/topology", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", apiKey)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("⚠️  Backend bağlantı hatası: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 || resp.StatusCode == 202 {
		fmt.Println("✅ Topoloji başarıyla sisteme aktarıldı ve ekranda güncellendi!")
	} else {
		fmt.Printf("⚠️  Backend beklenmedik bir kod döndürdü: %d\n", resp.StatusCode)
	}
}
