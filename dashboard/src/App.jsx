import { useState, useEffect, useRef } from 'react'

function App() {
    const [key, setKey] = useState('user:harsh123')
    const [ruleId] = useState('pro-tier')
    const [quota, setQuota] = useState(null)
    const [connected, setConnected] = useState(false)
    const esRef = useRef(null)

    const connect = () => {
        if (esRef.current) esRef.current.close()
        
        const es = new EventSource(`http://localhost:8080/quota?key=${key}`)
        esRef.current = es
        setConnected(true)

        es.onmessage = (e) => {
            const data = JSON.parse(e.data)
            setQuota(data)
        }

        es.onerror = () => setConnected(false)
    }

    const fireRequests = async (count = 20) => {
        for (let i = 0; i < count; i++) {
            await fetch(`http://localhost:8080/fire?key=${key}&rule_id=${ruleId}`)
            await new Promise(r => setTimeout(r, 50))
        }
    }

    const pct = quota ? Math.max(0, (quota.Remaining / quota.Limit) * 100) : 100

    return (
        <div style={{ maxWidth: 500, margin: '40px auto', fontFamily: 'monospace' }}>
            <h2>Rate Limiter Dashboard</h2>
            
            <div>
                <input value={key} onChange={e => setKey(e.target.value)} style={{ width: '300px' }} />
                <button onClick={connect}>Connect</button>
                <span>{connected ? ' 🟢 connected' : ' 🔴 disconnected'}</span>
            </div>

            {quota && (
                <div style={{ marginTop: 20 }}>
                    <p>Used: {quota.Used} / {quota.Limit}</p>
                    <p>Remaining: {quota.Remaining}</p>
                    <p>Exceeded: {quota.Exceeded ? '❌ YES' : '✅ NO'}</p>
                    <div style={{ background: '#eee', height: 20, borderRadius: 4, overflow: 'hidden' }}>
                        <div style={{
                            background: quota.Exceeded ? 'red' : 'green',
                            height: '100%',
                            width: `${pct}%`,
                            transition: 'width 0.2s'
                        }} />
                    </div>
                </div>
            )}

            <div style={{ marginTop: 20 }}>
                <button onClick={() => fireRequests(5)}>Fire 5 requests</button>
                <button onClick={() => fireRequests(20)} style={{ marginLeft: 8 }}>Fire 20 requests</button>
            </div>
        </div>
    )
}

export default App