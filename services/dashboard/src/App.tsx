import React, { useState, useEffect } from 'react';
import { User, MCPServer } from './types';
import { Login } from './components/Login';
import { ServersTab } from './components/ServersTab';
import { KeysTab } from './components/KeysTab';
import { MembersTab } from './components/MembersTab';
import { AuditTab } from './components/AuditTab';
import { ChatTab } from './components/ChatTab';
import { ConnectTab } from './components/ConnectTab';

export const App: React.FC = () => {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<'servers' | 'chat' | 'keys' | 'audit' | 'members' | 'connect'>('servers');
  const [servers, setServers] = useState<MCPServer[]>([]);

  const checkAuth = async () => {
    try {
      const res = await fetch('/api/auth/me');
      if (res.ok) {
        const u = await res.json();
        setUser(u);
      } else {
        setUser(null);
      }
    } catch {
      setUser(null);
    } finally {
      setLoading(false);
    }
  };

  const fetchServers = async () => {
    try {
      const res = await fetch('/api/servers');
      if (res.ok) {
        const data = await res.json();
        setServers(data);
      }
    } catch (err) {
      console.error(err);
    }
  };

  useEffect(() => {
    checkAuth();
  }, []);

  useEffect(() => {
    if (user) {
      fetchServers();
      const interval = setInterval(fetchServers, 5000);
      return () => clearInterval(interval);
    }
  }, [user]);

  const handleLogout = async () => {
    await fetch('/api/auth/logout', { method: 'POST' });
    setUser(null);
  };

  if (loading) {
    return (
      <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#9ca3af' }}>
        Loading Urumi Gateway...
      </div>
    );
  }

  if (!user) {
    return <Login onLoginSuccess={(u) => setUser(u)} />;
  }

  const navItems = [
    { id: 'servers', label: 'Servers' },
    { id: 'chat', label: 'Chat Agent' },
    { id: 'keys', label: 'API Keys' },
    { id: 'audit', label: 'Audit Log' },
    ...(user.role === 'admin' ? [{ id: 'members', label: 'Team Members' }] : []),
    { id: 'connect', label: 'Connect Clients' },
  ];

  return (
    <div style={{ minHeight: '100vh', display: 'flex', flexDirection: 'column' }}>
      {/* Top Navbar */}
      <header style={{ background: '#090d16', borderBottom: '1px solid #1f2937', padding: '0 2rem', height: '64px', display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '2.5rem' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
            <div style={{ width: '32px', height: '32px', borderRadius: '8px', background: 'linear-gradient(135deg, #3b82f6, #1d4ed8)', display: 'flex', alignItems: 'center', justifyContent: 'center', fontWeight: 'bold', color: '#fff' }}>
              U
            </div>
            <span style={{ fontWeight: 700, fontSize: '1.15rem', letterSpacing: '-0.02em', background: 'linear-gradient(135deg, #fff, #9ca3af)', WebkitBackgroundClip: 'text', WebkitTextFillColor: 'transparent' }}>
              Urumi Gateway
            </span>
          </div>

          <nav style={{ display: 'flex', gap: '0.5rem' }}>
            {navItems.map((item) => {
              const active = activeTab === item.id;
              return (
                <button
                  key={item.id}
                  onClick={() => setActiveTab(item.id as any)}
                  style={{
                    padding: '0.45rem 0.9rem',
                    background: active ? '#1f2937' : 'transparent',
                    color: active ? '#60a5fa' : '#9ca3af',
                    border: 'none',
                    borderRadius: '6px',
                    fontSize: '0.85rem',
                    fontWeight: active ? 600 : 500,
                    cursor: 'pointer',
                    transition: 'all 0.15s ease',
                  }}
                >
                  {item.label}
                </button>
              );
            })}
          </nav>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
          <span style={{ fontSize: '0.8rem', color: '#9ca3af' }}>{user.email}</span>
          <span style={{ background: user.role === 'admin' ? 'rgba(59,130,246,0.2)' : 'rgba(107,114,128,0.2)', color: user.role === 'admin' ? '#60a5fa' : '#9ca3af', padding: '0.2rem 0.5rem', borderRadius: '4px', fontSize: '0.7rem', fontWeight: 600 }}>
            {user.role.toUpperCase()}
          </span>
          <button
            onClick={handleLogout}
            style={{ padding: '0.35rem 0.75rem', background: 'transparent', color: '#ef4444', border: '1px solid rgba(239,68,68,0.3)', borderRadius: '6px', fontSize: '0.75rem', cursor: 'pointer' }}
          >
            Sign Out
          </button>
        </div>
      </header>

      {/* Main Content Area */}
      <main style={{ flex: 1, padding: '2rem', maxWidth: '1280px', margin: '0 auto', width: '100%' }}>
        {activeTab === 'servers' && (
          <ServersTab servers={servers} currentUser={user} onRefresh={fetchServers} />
        )}
        {activeTab === 'chat' && <ChatTab />}
        {activeTab === 'keys' && <KeysTab />}
        {activeTab === 'audit' && <AuditTab />}
        {activeTab === 'members' && user.role === 'admin' && <MembersTab />}
        {activeTab === 'connect' && <ConnectTab />}
      </main>
    </div>
  );
};
