import React, { useState, useEffect } from 'react';
import { AuditRecord } from '../types';

export const AuditTab: React.FC = () => {
  const [logs, setLogs] = useState<AuditRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [outcomeFilter, setOutcomeFilter] = useState('');
  const [serverFilter, setServerFilter] = useState('');
  const [autoRefresh, setAutoRefresh] = useState(true);

  const fetchLogs = async () => {
    try {
      const q = new URLSearchParams();
      if (outcomeFilter) q.set('outcome', outcomeFilter);
      if (serverFilter) q.set('server', serverFilter);

      const res = await fetch(`/api/audit?${q.toString()}`);
      if (res.ok) {
        const data = await res.json();
        setLogs(data);
      }
    } catch (err) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchLogs();
  }, [outcomeFilter, serverFilter]);

  useEffect(() => {
    if (!autoRefresh) return;
    const interval = setInterval(fetchLogs, 5000);
    return () => clearInterval(interval);
  }, [autoRefresh, outcomeFilter, serverFilter]);

  const getOutcomeStyle = (outcome: string) => {
    switch (outcome) {
      case 'success':
        return { bg: 'rgba(16,185,129,0.15)', color: '#10b981' };
      case 'timeout':
        return { bg: 'rgba(245,158,11,0.15)', color: '#f59e0b' };
      case 'unreachable':
        return { bg: 'rgba(239,68,68,0.15)', color: '#ef4444' };
      case 'auth_rejected':
      case 'denied':
        return { bg: 'rgba(168,85,247,0.15)', color: '#c084fc' };
      default:
        return { bg: 'rgba(239,68,68,0.15)', color: '#f87171' };
    }
  };

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1.5rem' }}>
        <div>
          <h2 style={{ margin: 0, fontSize: '1.5rem', fontWeight: 600 }}>Audit Ledger</h2>
          <p style={{ margin: '0.25rem 0 0 0', color: '#9ca3af', fontSize: '0.875rem' }}>
            Append-only record of every tool invocation. Attributes user, channel, server, duration, and outcome.
          </p>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
          <label style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', fontSize: '0.875rem', color: '#9ca3af', cursor: 'pointer' }}>
            <input
              type="checkbox"
              checked={autoRefresh}
              onChange={(e) => setAutoRefresh(e.target.checked)}
            />
            Auto-refresh (5s)
          </label>

          <select
            value={outcomeFilter}
            onChange={(e) => setOutcomeFilter(e.target.value)}
            style={{ padding: '0.4rem 0.75rem', background: '#111827', border: '1px solid #374151', borderRadius: '6px', color: '#fff', fontSize: '0.8rem' }}
          >
            <option value="">All Outcomes</option>
            <option value="success">Success</option>
            <option value="timeout">Timeout</option>
            <option value="unreachable">Unreachable</option>
            <option value="tool_error">Tool Error</option>
            <option value="denied">Denied</option>
            <option value="auth_rejected">Auth Rejected</option>
          </select>

          <input
            placeholder="Filter server slug..."
            value={serverFilter}
            onChange={(e) => setServerFilter(e.target.value)}
            style={{ padding: '0.4rem 0.75rem', background: '#111827', border: '1px solid #374151', borderRadius: '6px', color: '#fff', fontSize: '0.8rem', width: '140px' }}
          />

          <button
            onClick={fetchLogs}
            style={{ padding: '0.4rem 0.75rem', background: '#1f2937', color: '#d1d5db', border: 'none', borderRadius: '6px', fontSize: '0.8rem', cursor: 'pointer' }}
          >
            Refresh
          </button>
        </div>
      </div>

      {loading ? (
        <div style={{ color: '#9ca3af', fontSize: '0.875rem' }}>Loading audit records...</div>
      ) : logs.length === 0 ? (
        <div style={{ background: '#111827', border: '1px solid #1f2937', borderRadius: '10px', padding: '2rem', textAlign: 'center', color: '#9ca3af' }}>
          No audit records found matching filters.
        </div>
      ) : (
        <div style={{ background: '#111827', border: '1px solid #1f2937', borderRadius: '10px', overflow: 'hidden' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', textAlign: 'left', fontSize: '0.8rem' }}>
            <thead>
              <tr style={{ background: '#090d16', borderBottom: '1px solid #1f2937', color: '#9ca3af' }}>
                <th style={{ padding: '0.75rem 1rem' }}>Timestamp</th>
                <th style={{ padding: '0.75rem 1rem' }}>User</th>
                <th style={{ padding: '0.75rem 1rem' }}>Channel</th>
                <th style={{ padding: '0.75rem 1rem' }}>Exposed Tool</th>
                <th style={{ padding: '0.75rem 1rem' }}>Duration</th>
                <th style={{ padding: '0.75rem 1rem' }}>Outcome</th>
                <th style={{ padding: '0.75rem 1rem' }}>Error / Code</th>
              </tr>
            </thead>
            <tbody>
              {logs.map((r) => {
                const badge = getOutcomeStyle(r.outcome);
                return (
                  <tr key={r.id} style={{ borderBottom: '1px solid #1f2937' }}>
                    <td style={{ padding: '0.6rem 1rem', color: '#9ca3af', whiteSpace: 'nowrap' }}>
                      {new Date(r.ts).toLocaleTimeString()} ({new Date(r.ts).toLocaleDateString()})
                    </td>
                    <td style={{ padding: '0.6rem 1rem', fontWeight: 500, color: '#f3f4f6' }}>
                      {r.user_email}
                    </td>
                    <td style={{ padding: '0.6rem 1rem' }}>
                      <span style={{ background: r.channel === 'mcp' ? '#1e3a8a' : '#14532d', color: '#bfdbfe', padding: '0.15rem 0.4rem', borderRadius: '4px', fontSize: '0.7rem', fontWeight: 600 }}>
                        {r.channel.toUpperCase()}
                      </span>
                    </td>
                    <td style={{ padding: '0.6rem 1rem', fontFamily: 'monospace', color: '#60a5fa' }}>
                      {r.exposed_name}
                    </td>
                    <td style={{ padding: '0.6rem 1rem', color: '#9ca3af' }}>
                      {r.duration_ms} ms
                    </td>
                    <td style={{ padding: '0.6rem 1rem' }}>
                      <span style={{ background: badge.bg, color: badge.color, padding: '0.2rem 0.5rem', borderRadius: '10px', fontSize: '0.7rem', fontWeight: 600 }}>
                        ● {r.outcome}
                      </span>
                    </td>
                    <td style={{ padding: '0.6rem 1rem', color: '#f87171', fontFamily: 'monospace', fontSize: '0.75rem' }}>
                      {r.error_code || '—'}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
};
