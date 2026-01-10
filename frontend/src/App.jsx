import { useState, useEffect } from 'react'

function App() {
  const [taskName, setTaskName] = useState("");
  const [tasks, setTasks] = useState([]);

  // 1. Uygulama açılınca WebSocket'e bağlan
  useEffect(() => {
    // Backend'deki /ws adresine bağlanıyoruz
    const socket = new WebSocket("ws://localhost:8080/ws");

    socket.onopen = () => {
      console.log("Bağlantı başarılı ✅");
    };

    // 2. Mesaj Geldiğinde Ne Yapacağız?
    socket.onmessage = (event) => {
      const result = JSON.parse(event.data);
      console.log("Backend'den mesaj geldi:", result);

      // Listemizdeki ilgili görevi bulup durumunu güncelle
      setTasks((prevTasks) => 
        prevTasks.map((task) => 
          task.id === result.id 
            ? { ...task, status: result.status, detail: result.message } 
            : task
        )
      );
    };

    return () => socket.close();
  }, []);

  const addTask = async () => {
    if (!taskName) return;
    try {
      const response = await fetch(`http://localhost:8080/add-task?name=${taskName}`);
      const data = await response.json();
      // İlk başta "İşleniyor..." olarak ekle
      setTasks([...tasks, { ...data.task, status: "İşleniyor... ⏳" }]);
      setTaskName("");
    } catch (error) {
      console.error(error);
    }
  };

  return (
    <div style={{ padding: '40px', fontFamily: 'Segoe UI, sans-serif', maxWidth: '600px', margin: '0 auto' }}>
      <h1>Concurrent Log Streamer</h1>
      
      <div style={{ display: 'flex', gap: '10px', marginBottom: '20px' }}>
        <input 
          style={{ padding: '10px', flex: 1, borderRadius: '5px', border: '1px solid #ccc' }}
          value={taskName} 
          onChange={(e) => setTaskName(e.target.value)}
          placeholder="Örn: Veri Analizi..." 
        />
        <button 
          onClick={addTask}
          style={{ padding: '10px 20px', background: '#007bff', color: 'white', border: 'none', borderRadius: '5px', cursor: 'pointer' }}
        >
          Başlat
        </button>
      </div>

      <ul style={{ listStyle: 'none', padding: 0 }}>
        {tasks.map((t) => (
          <li key={t.id} style={{ 
            padding: '15px', 
            marginBottom: '10px', 
            background: t.status.includes("Tamamlandı") ? '#d4edda' : '#fff3cd',
            borderRadius: '8px',
            border: '1px solid #ddd',
            transition: 'background 0.5s ease'
          }}>
            <div style={{ fontWeight: 'bold', fontSize: '1.1em' }}>{t.name} <small>(ID: {t.id})</small></div>
            <div style={{ color: '#555', marginTop: '5px' }}>
              Durum: <strong>{t.status}</strong>
            </div>
            {t.detail && <div style={{ fontSize: '0.8em', color: '#666' }}>{t.detail}</div>}
          </li>
        ))}
      </ul>
    </div>
  )
}

export default App