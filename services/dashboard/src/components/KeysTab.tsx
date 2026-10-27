import React, { useState, useEffect } from 'react';
import { APIKey } from '../types';

export const KeysTab: React.FC = () => {
  const [keys, setKeys] = useState<APIKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [newKeyName, setNewKeyName] = useState('');
  const [createdKey, setCreatedKey] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  const fetchKeys = async () => {
    try {
      const res = await fetch('/api/keys');
      if (res.ok) {
        const data = await res.json();
        setKeys(data);
      }
    } catch (err) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchKeys();
  }, []);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const res = await fetch('/api/keys', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: newKeyName }),
      });
      if (res.ok) {
        const data = await res.json();
        setCreatedKey(data.api_key);
        fetchKeys();
      }
    } catch (err) {
      console.error(err);
    }
  };

  const handleRevoke = async (id: number) => {
    if (!confirm('Are you sure you want to revoke this API key? Any client using it will immediately be rejected with 401.')) return;
    try {
      await fetch(`/api/keys/${id}`, { method: 'DELETE' });
      fetchKeys();
    } catch (err) {
      console.error(err);
    }
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1.5rem' }}>
        <div>
          <h2 style={{ margin: 0, fontSize: '1.5rem', fontWeight: 600 }}>Personal API Keys</h2>
          <p style={{ margin: '0.25rem 0 0 0', color: '#9ca3af', fontSize: '0.875rem' }}>
            High-entropy keys for external clients (Claude Code, Cursor, MCP Inspector). Checked directly against PostgreSQL on every request.
          </p>
        </div>
        <button
          onClick={() => {
            setShowCreateModal(true);
            setCreatedKey(null);
            setNewKeyName('');
          }}
          style={{ padding: '0.5rem 1rem', background: '#3b82f6', color: '#fff', border: 'none', borderRadius: '6px', fontWeight: 500, cursor: 'pointer', fontSize: '0.875rem' }}
        >
          + Create API Key
        </button>
      </div>

      {loading ? (
        <div style={{ color: '#9ca3af', fontSize: '0.875rem' }}>Loading keys...</div>
      ) : keys.length === 0 ? (
        <div style={{ background: '#111827', border: '1px solid #1f2937', borderRadius: '10px', padding: '2rem', textAlign: 'center', color: '#9ca3af' }}>
          No API keys generated yet. Click "+ Create API Key" to connect your external MCP client.
        </div>
      ) : (
        <div style={{ background: '#111827', border: '1px solid #1f2937', borderRadius: '10px', overflow: 'hidden' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', textAlign: 'left', fontSize: '0.875rem' }}>
            <thead>
              <tr style={{ background: '#090d16', borderBottom: '1px solid #1f2937', color: '#9ca3af' }}>
                <th style={{ padding: '0.75rem 1rem' }}>Name</th>
                <th style={{ padding: '0.75rem 1rem' }}>Key Prefix</th>
                <th style={{ padding: '0.75rem 1rem' }}>Owner</th>
                <th style={{ padding: '0.75rem 1rem' }}>Created</th>
                <th style={{ padding: '0.75rem 1rem' }}>Last Used</th>
                <th style={{ padding: '0.75rem 1rem' }}>Status</th>
                <th style={{ padding: '0.75rem 1rem', textAlign: 'right' }}>Actions</th>
              </tr>
            </thead>
            <tbody>
              {keys.map((k) => (
                <tr key={k.id} style={{ borderBottom: '1px solid #1f2937' }}>
                  <td style={{ padding: '0.75rem 1rem', fontWeight: 500 }}>{k.name}</td>
                  <td style={{ padding: '0.75rem 1rem', fontFamily: 'monospace', color: '#60a5fa' }}>
                    urmk_{k.prefix}_••••
                  </td>
                  <td style={{ padding: '0.75rem 1rem', color: '#9ca3af' }}>{k.user_email}</td>
                  <td style={{ padding: '0.75rem 1rem', color: '#9ca3af' }}>
                    {new Date(k.created_at).toLocaleDateString()}
                  </td>
                  <td style={{ padding: '0.75rem 1rem', color: '#9ca3af' }}>
                    {k.last_used_at ? new Date(k.last_used_at).toLocaleTimeString() : 'Never'}
                  </td>
                  <td style={{ padding: '0.75rem 1rem' }}>
                    {k.revoked_at ? (
                      <span style={{ background: 'rgba(239,68,68,0.15)', color: '#ef4444', padding: '0.2rem 0.5rem', borderRadius: '10px', fontSize: '0.75rem', fontWeight: 600 }}>
                        Revoked
                      </span>
                    ) : (
                      <span style={{ background: 'rgba(16,185,129,0.15)', color: '#10b981', padding: '0.2rem 0.5rem', borderRadius: '10px', fontSize: '0.75rem', fontWeight: 600 }}>
                        Active
                      </span>
                    )}
                  </td>
                  <td style={{ padding: '0.75rem 1rem', textAlign: 'right' }}>
                    {!k.revoked_at && (
                      <button
                        onClick={() => handleRevoke(k.id)}
                        style={{ padding: '0.25rem 0.6rem', background: 'rgba(239,68,68,0.2)', color: '#f87171', border: 'none', borderRadius: '4px', fontSize: '0.75rem', cursor: 'pointer' }}
                      >
                        Revoke
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {showCreateModal && (
        <div style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.7)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 50, padding: '1rem' }}>
          <div style={{ background: '#111827', border: '1px solid #1f2937', borderRadius: '12px', padding: '2rem', width: '100%', maxWidth: '480px' }}>
            <h3 style={{ margin: '0 0 1rem 0', fontSize: '1.25rem' }}>
              {createdKey ? 'API Key Created' : 'Create New API Key'}
            </h3>

            {!createdKey ? (
              <form onSubmit={handleCreate}>
                <div style={{ marginBottom: '1.25rem' }}>
                  <label style={{ display: 'block', fontSize: '0.875rem', color: '#d1d5db', marginBottom: '0.5rem' }}>Key Name / Client</label>
                  <input
                    placeholder="e.g. Claude Code (MacBook Pro)"
                    value={newKeyName}
                    onChange={(e) => setNewKeyName(e.target.value)}
                    required
                    style={{ width: '100%', padding: '0.6rem 0.8rem', background: '#090d16', border: '1px solid #374151', borderRadius: '6px', color: '#fff', fontSize: '0.875rem' }}
                  />
                </div>
                <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '0.5rem' }}>
                  <button
                    type="button"
                    onClick={() => setShowCreateModal(false)}
                    style={{ padding: '0.5rem 1rem', background: 'transparent', color: '#9ca3af', border: '1px solid #374151', borderRadius: '6px', cursor: 'pointer' }}
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    style={{ padding: '0.5rem 1rem', background: '#3b82f6', color: '#fff', border: 'none', borderRadius: '6px', fontWeight: 600, cursor: 'pointer' }}
                  >
                    Generate
                  </button>
                </div>
              </form>
            ) : (
              <div>
                <p style={{ color: '#f59e0b', fontSize: '0.875rem', margin: '0 0 1rem 0' }}>
                  ⚠️ Make sure to copy your API key now. You will not be able to see it again!
                </p>

                <div style={{ background: '#090d16', border: '1px solid #374151', borderRadius: '6px', padding: '0.75rem', display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1.5rem' }}>
                  <code style={{ color: '#60a5fa', fontSize: '0.8rem', wordBreak: 'break-all' }}>{createdKey}</code>
                  <button
                    onClick={() => copyToClipboard(createdKey)}
                    style={{ padding: '0.35rem 0.75rem', background: copied ? '#10b981' : '#3b82f6', color: '#fff', border: 'none', borderRadius: '4px', fontSize: '0.75rem', cursor: 'pointer', marginLeft: '0.5rem', whiteSpace: 'nowrap' }}
                  >
                    {copied ? 'Copied!' : 'Copy'}
                  </button>
                </div>

                <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
                  <button
                    onClick={() => setShowCreateModal(false)}
                    style={{ padding: '0.5rem 1rem', background: '#374151', color: '#fff', border: 'none', borderRadius: '6px', cursor: 'pointer', fontSize: '0.875rem' }}
                  >
                    Done
                  </button>
                </div>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
};
