import React, { useState } from 'react';
import { MCPServer, User } from '../types';

interface ServersTabProps {
  servers: MCPServer[];
  currentUser: User;
  onRefresh: () => void;
}

export const ServersTab: React.FC<ServersTabProps> = ({ servers, currentUser, onRefresh }) => {
  const [expandedServer, setExpandedServer] = useState<string | null>(null);
  const [showAddModal, setShowAddModal] = useState(false);
  const [newSlug, setNewSlug] = useState('');
  const [newName, setNewName] = useState('');
  const [newUrl, setNewUrl] = useState('');
  const [newAuthKind, setNewAuthKind] = useState('bearer');
  const [newAuthHeader, setNewAuthHeader] = useState('');
  const [newCredential, setNewCredential] = useState('');
  const [testResult, setTestResult] = useState<{ ok: boolean; message: string } | null>(null);
  const [testing, setTesting] = useState(false);
  const [saving, setSaving] = useState(false);

  const toggleEnable = async (server: MCPServer) => {
    try {
      await fetch(`/api/servers/${server.id}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ enabled: !server.enabled }),
      });
      onRefresh();
    } catch (err) {
      console.error(err);
    }
  };

  const deleteServer = async (id: number) => {
    if (!confirm('Are you sure you want to remove this upstream server?')) return;
    try {
      await fetch(`/api/servers/${id}`, { method: 'DELETE' });
      onRefresh();
    } catch (err) {
      console.error(err);
    }
  };

  const handleTestConnection = async () => {
    setTesting(true);
    setTestResult(null);
    try {
      const res = await fetch('/api/servers/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          url: newUrl,
          auth_kind: newAuthKind,
          auth_header: newAuthHeader,
          credential: newCredential,
        }),
      });
      const data = await res.json();
      if (data.ok) {
        setTestResult({ ok: true, message: `Connected successfully! Discovered ${data.tools_count} tools.` });
      } else {
        setTestResult({ ok: false, message: `Connection failed: ${data.error}` });
      }
    } catch (err: any) {
      setTestResult({ ok: false, message: err.message || 'Network test failed' });
    } finally {
      setTesting(false);
    }
  };

  const handleAddSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    try {
      const res = await fetch('/api/servers', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          slug: newSlug,
          name: newName,
          url: newUrl,
          auth_kind: newAuthKind,
          auth_header: newAuthHeader,
          credential: newCredential,
        }),
      });
      if (!res.ok) {
        const d = await res.json();
        throw new Error(d.error || 'Failed to add server');
      }
      setShowAddModal(false);
      setNewSlug('');
      setNewName('');
      setNewUrl('');
      setNewCredential('');
      setTestResult(null);
      onRefresh();
    } catch (err: any) {
      alert(err.message);
    } finally {
      setSaving(false);
    }
  };

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1.5rem' }}>
        <div>
          <h2 style={{ margin: 0, fontSize: '1.5rem', fontWeight: 600 }}>Upstream MCP Servers</h2>
          <p style={{ margin: '0.25rem 0 0 0', color: '#9ca3af', fontSize: '0.875rem' }}>
            Registered tool providers aggregated into the gateway's unified <code style={{ color: '#60a5fa' }}>/mcp</code> endpoint.
          </p>
        </div>
        {currentUser.role === 'admin' && (
          <button
            onClick={() => setShowAddModal(true)}
            style={{ padding: '0.5rem 1rem', background: '#3b82f6', color: '#fff', border: 'none', borderRadius: '6px', fontWeight: 500, cursor: 'pointer', fontSize: '0.875rem' }}
          >
            + Add Server
          </button>
        )}
      </div>

      <div style={{ display: 'grid', gap: '1rem' }}>
        {servers.map((s) => {
          const isExpanded = expandedServer === s.slug;
          const statusBg = s.status === 'Connected' ? 'rgba(16, 185, 129, 0.15)' : s.status === 'Unreachable' ? 'rgba(239, 68, 68, 0.15)' : 'rgba(156, 163, 175, 0.15)';
          const statusColor = s.status === 'Connected' ? '#10b981' : s.status === 'Unreachable' ? '#ef4444' : '#9ca3af';

          return (
            <div key={s.id} style={{ background: '#111827', border: '1px solid #1f2937', borderRadius: '10px', padding: '1.25rem' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
                  <span style={{ fontWeight: 600, fontSize: '1.1rem' }}>{s.name}</span>
                  <span style={{ fontSize: '0.75rem', background: '#1f2937', padding: '0.2rem 0.5rem', borderRadius: '4px', color: '#9ca3af', fontFamily: 'monospace' }}>
                    slug: {s.slug}
                  </span>
                  <span style={{ fontSize: '0.75rem', padding: '0.2rem 0.6rem', borderRadius: '12px', background: statusBg, color: statusColor, fontWeight: 600 }}>
                    ● {s.status}
                  </span>
                  {s.status === 'Unreachable' && s.reason && (
                    <span style={{ fontSize: '0.75rem', color: '#f87171', background: 'rgba(239, 68, 68, 0.1)', padding: '0.1rem 0.4rem', borderRadius: '4px' }}>
                      reason: {s.reason}
                    </span>
                  )}
                </div>

                <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
                  {currentUser.role === 'admin' && (
                    <>
                      <button
                        onClick={() => toggleEnable(s)}
                        style={{ padding: '0.35rem 0.75rem', background: s.enabled ? '#374151' : '#10b981', color: '#fff', border: 'none', borderRadius: '6px', fontSize: '0.75rem', cursor: 'pointer' }}
                      >
                        {s.enabled ? 'Disable' : 'Enable'}
                      </button>
                      <button
                        onClick={() => deleteServer(s.id)}
                        style={{ padding: '0.35rem 0.75rem', background: 'rgba(239, 68, 68, 0.2)', color: '#f87171', border: 'none', borderRadius: '6px', fontSize: '0.75rem', cursor: 'pointer' }}
                      >
                        Delete
                      </button>
                    </>
                  )}
                  <button
                    onClick={() => setExpandedServer(isExpanded ? null : s.slug)}
                    style={{ padding: '0.35rem 0.75rem', background: '#1f2937', color: '#d1d5db', border: 'none', borderRadius: '6px', fontSize: '0.75rem', cursor: 'pointer' }}
                  >
                    {isExpanded ? 'Hide Tools' : `Tools (${s.tools.length})`}
                  </button>
                </div>
              </div>

              {s.url && (
                <div style={{ marginTop: '0.5rem', fontSize: '0.8rem', color: '#6b7280', fontFamily: 'monospace' }}>
                  Target: {s.url}
                </div>
              )}

              {isExpanded && (
                <div style={{ marginTop: '1rem', borderTop: '1px solid #1f2937', paddingTop: '1rem' }}>
                  <h4 style={{ margin: '0 0 0.75rem 0', fontSize: '0.875rem', color: '#9ca3af' }}>Advertised Tools ({s.tools.length})</h4>
                  {s.tools.length === 0 ? (
                    <p style={{ color: '#6b7280', fontSize: '0.85rem' }}>No tools advertised or server currently unreachable.</p>
                  ) : (
                    <div style={{ display: 'grid', gap: '0.75rem' }}>
                      {s.tools.map((t) => (
                        <div key={t.name} style={{ background: '#090d16', border: '1px solid #1f2937', borderRadius: '6px', padding: '0.75rem' }}>
                          <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', marginBottom: '0.25rem' }}>
                            <code style={{ color: '#60a5fa', fontWeight: 600, fontSize: '0.875rem' }}>
                              {s.slug}__{t.name}
                            </code>
                          </div>
                          <p style={{ margin: '0 0 0.5rem 0', fontSize: '0.8rem', color: '#9ca3af' }}>{t.description}</p>
                          {t.inputSchema && (
                            <pre style={{ margin: 0, padding: '0.5rem', background: '#111827', borderRadius: '4px', fontSize: '0.75rem', color: '#d1d5db', overflowX: 'auto' }}>
                              {JSON.stringify(t.inputSchema, null, 2)}
                            </pre>
                          )}
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              )}
            </div>
          );
        })}
      </div>

      {showAddModal && (
        <div style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.7)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 50, padding: '1rem' }}>
          <div style={{ background: '#111827', border: '1px solid #1f2937', borderRadius: '12px', padding: '2rem', width: '100%', maxWidth: '520px' }}>
            <h3 style={{ margin: '0 0 1.5rem 0', fontSize: '1.25rem' }}>Add Upstream MCP Server</h3>
            <form onSubmit={handleAddSubmit}>
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 2fr', gap: '1rem', marginBottom: '1rem' }}>
                <div>
                  <label style={{ display: 'block', fontSize: '0.75rem', color: '#9ca3af', marginBottom: '0.25rem' }}>Slug (prefix)</label>
                  <input
                    placeholder="e.g. docs"
                    value={newSlug}
                    onChange={(e) => setNewSlug(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, ''))}
                    required
                    style={{ width: '100%', padding: '0.5rem', background: '#090d16', border: '1px solid #374151', borderRadius: '6px', color: '#fff', fontSize: '0.875rem' }}
                  />
                </div>
                <div>
                  <label style={{ display: 'block', fontSize: '0.75rem', color: '#9ca3af', marginBottom: '0.25rem' }}>Display Name</label>
                  <input
                    placeholder="e.g. Documentation MCP"
                    value={newName}
                    onChange={(e) => setNewName(e.target.value)}
                    required
                    style={{ width: '100%', padding: '0.5rem', background: '#090d16', border: '1px solid #374151', borderRadius: '6px', color: '#fff', fontSize: '0.875rem' }}
                  />
                </div>
              </div>

              <div style={{ marginBottom: '1rem' }}>
                <label style={{ display: 'block', fontSize: '0.75rem', color: '#9ca3af', marginBottom: '0.25rem' }}>MCP Streamable HTTP Endpoint URL</label>
                <input
                  placeholder="https://.../mcp or http://service.namespace:8080/mcp"
                  value={newUrl}
                  onChange={(e) => setNewUrl(e.target.value)}
                  required
                  style={{ width: '100%', padding: '0.5rem', background: '#090d16', border: '1px solid #374151', borderRadius: '6px', color: '#fff', fontSize: '0.875rem' }}
                />
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem', marginBottom: '1rem' }}>
                <div>
                  <label style={{ display: 'block', fontSize: '0.75rem', color: '#9ca3af', marginBottom: '0.25rem' }}>Auth Type</label>
                  <select
                    value={newAuthKind}
                    onChange={(e) => setNewAuthKind(e.target.value)}
                    style={{ width: '100%', padding: '0.5rem', background: '#090d16', border: '1px solid #374151', borderRadius: '6px', color: '#fff', fontSize: '0.875rem' }}
                  >
                    <option value="none">None (Public)</option>
                    <option value="bearer">Bearer Token</option>
                    <option value="header">Custom Header</option>
                  </select>
                </div>
                {newAuthKind === 'header' && (
                  <div>
                    <label style={{ display: 'block', fontSize: '0.75rem', color: '#9ca3af', marginBottom: '0.25rem' }}>Header Name</label>
                    <input
                      placeholder="X-API-Key"
                      value={newAuthHeader}
                      onChange={(e) => setNewAuthHeader(e.target.value)}
                      style={{ width: '100%', padding: '0.5rem', background: '#090d16', border: '1px solid #374151', borderRadius: '6px', color: '#fff', fontSize: '0.875rem' }}
                    />
                  </div>
                )}
              </div>

              {newAuthKind !== 'none' && (
                <div style={{ marginBottom: '1rem' }}>
                  <label style={{ display: 'block', fontSize: '0.75rem', color: '#9ca3af', marginBottom: '0.25rem' }}>Secret Credential (Encrypted at rest)</label>
                  <input
                    type="password"
                    placeholder="Credential value"
                    value={newCredential}
                    onChange={(e) => setNewCredential(e.target.value)}
                    style={{ width: '100%', padding: '0.5rem', background: '#090d16', border: '1px solid #374151', borderRadius: '6px', color: '#fff', fontSize: '0.875rem' }}
                  />
                </div>
              )}

              {testResult && (
                <div style={{ padding: '0.75rem', borderRadius: '6px', fontSize: '0.8rem', marginBottom: '1rem', background: testResult.ok ? 'rgba(16,185,129,0.1)' : 'rgba(239,68,68,0.1)', color: testResult.ok ? '#10b981' : '#ef4444', border: `1px solid ${testResult.ok ? 'rgba(16,185,129,0.2)' : 'rgba(239,68,68,0.2)'}` }}>
                  {testResult.message}
                </div>
              )}

              <div style={{ display: 'flex', justifyContent: 'space-between', marginTop: '1.5rem' }}>
                <button
                  type="button"
                  onClick={handleTestConnection}
                  disabled={testing || !newUrl}
                  style={{ padding: '0.5rem 1rem', background: '#374151', color: '#d1d5db', border: 'none', borderRadius: '6px', fontSize: '0.875rem', cursor: testing ? 'not-allowed' : 'pointer' }}
                >
                  {testing ? 'Pinging...' : 'Test Connection'}
                </button>

                <div style={{ display: 'flex', gap: '0.5rem' }}>
                  <button
                    type="button"
                    onClick={() => setShowAddModal(false)}
                    style={{ padding: '0.5rem 1rem', background: 'transparent', color: '#9ca3af', border: '1px solid #374151', borderRadius: '6px', fontSize: '0.875rem', cursor: 'pointer' }}
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    disabled={saving}
                    style={{ padding: '0.5rem 1rem', background: '#3b82f6', color: '#fff', border: 'none', borderRadius: '6px', fontSize: '0.875rem', fontWeight: 600, cursor: saving ? 'not-allowed' : 'pointer' }}
                  >
                    {saving ? 'Saving...' : 'Add Server'}
                  </button>
                </div>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
};
