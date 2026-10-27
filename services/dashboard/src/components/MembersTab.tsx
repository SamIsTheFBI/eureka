import React, { useState, useEffect } from 'react';
import { User } from '../types';

export const MembersTab: React.FC = () => {
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [showInviteModal, setShowInviteModal] = useState(false);
  const [inviteEmail, setInviteEmail] = useState('');
  const [inviteRole, setInviteRole] = useState<'member' | 'admin'>('member');
  const [inviteLink, setInviteLink] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  const fetchUsers = async () => {
    try {
      const res = await fetch('/api/users');
      if (res.ok) {
        const data = await res.json();
        setUsers(data);
      }
    } catch (err) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchUsers();
  }, []);

  const handleInvite = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const res = await fetch('/api/invites', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: inviteEmail, role: inviteRole }),
      });
      if (res.ok) {
        const data = await res.json();
        setInviteLink(data.invite_link);
        fetchUsers();
      }
    } catch (err) {
      console.error(err);
    }
  };

  const toggleUserActive = async (user: User) => {
    const action = user.active ? 'deactivate' : 'activate';
    if (!confirm(`Are you sure you want to ${action} user ${user.email}? ${user.active ? 'All active sessions and API keys will be immediately revoked.' : ''}`)) return;

    try {
      await fetch(`/api/users/${user.id}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ active: !user.active }),
      });
      fetchUsers();
    } catch (err) {
      console.error(err);
    }
  };

  const copyInvite = () => {
    if (inviteLink) {
      navigator.clipboard.writeText(inviteLink);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1.5rem' }}>
        <div>
          <h2 style={{ margin: 0, fontSize: '1.5rem', fontWeight: 600 }}>Team & Members</h2>
          <p style={{ margin: '0.25rem 0 0 0', color: '#9ca3af', fontSize: '0.875rem' }}>
            Control role-based access. Deactivating a member immediately revokes all their active API keys and sessions.
          </p>
        </div>
        <button
          onClick={() => {
            setShowInviteModal(true);
            setInviteLink(null);
            setInviteEmail('');
          }}
          style={{ padding: '0.5rem 1rem', background: '#3b82f6', color: '#fff', border: 'none', borderRadius: '6px', fontWeight: 500, cursor: 'pointer', fontSize: '0.875rem' }}
        >
          + Invite Member
        </button>
      </div>

      {loading ? (
        <div style={{ color: '#9ca3af', fontSize: '0.875rem' }}>Loading team members...</div>
      ) : (
        <div style={{ background: '#111827', border: '1px solid #1f2937', borderRadius: '10px', overflow: 'hidden' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', textAlign: 'left', fontSize: '0.875rem' }}>
            <thead>
              <tr style={{ background: '#090d16', borderBottom: '1px solid #1f2937', color: '#9ca3af' }}>
                <th style={{ padding: '0.75rem 1rem' }}>User Email</th>
                <th style={{ padding: '0.75rem 1rem' }}>Role</th>
                <th style={{ padding: '0.75rem 1rem' }}>Active Keys</th>
                <th style={{ padding: '0.75rem 1rem' }}>Joined</th>
                <th style={{ padding: '0.75rem 1rem' }}>Status</th>
                <th style={{ padding: '0.75rem 1rem', textAlign: 'right' }}>Actions</th>
              </tr>
            </thead>
            <tbody>
              {users.map((u) => (
                <tr key={u.id} style={{ borderBottom: '1px solid #1f2937' }}>
                  <td style={{ padding: '0.75rem 1rem', fontWeight: 500 }}>{u.email}</td>
                  <td style={{ padding: '0.75rem 1rem' }}>
                    <span style={{ background: u.role === 'admin' ? 'rgba(59,130,246,0.15)' : 'rgba(107,114,128,0.2)', color: u.role === 'admin' ? '#60a5fa' : '#d1d5db', padding: '0.2rem 0.5rem', borderRadius: '4px', fontSize: '0.75rem', fontWeight: 600 }}>
                      {u.role.toUpperCase()}
                    </span>
                  </td>
                  <td style={{ padding: '0.75rem 1rem', color: '#9ca3af' }}>{u.key_count ?? 0} keys</td>
                  <td style={{ padding: '0.75rem 1rem', color: '#9ca3af' }}>
                    {new Date(u.created_at).toLocaleDateString()}
                  </td>
                  <td style={{ padding: '0.75rem 1rem' }}>
                    {u.active ? (
                      <span style={{ color: '#10b981', fontWeight: 600, fontSize: '0.8rem' }}>● Active</span>
                    ) : (
                      <span style={{ color: '#ef4444', fontWeight: 600, fontSize: '0.8rem' }}>● Deactivated</span>
                    )}
                  </td>
                  <td style={{ padding: '0.75rem 1rem', textAlign: 'right' }}>
                    <button
                      onClick={() => toggleUserActive(u)}
                      style={{ padding: '0.25rem 0.6rem', background: u.active ? 'rgba(239,68,68,0.2)' : 'rgba(16,185,129,0.2)', color: u.active ? '#f87171' : '#10b981', border: 'none', borderRadius: '4px', fontSize: '0.75rem', cursor: 'pointer' }}
                    >
                      {u.active ? 'Deactivate' : 'Reactivate'}
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {showInviteModal && (
        <div style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.7)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 50, padding: '1rem' }}>
          <div style={{ background: '#111827', border: '1px solid #1f2937', borderRadius: '12px', padding: '2rem', width: '100%', maxWidth: '480px' }}>
            <h3 style={{ margin: '0 0 1rem 0', fontSize: '1.25rem' }}>
              {inviteLink ? 'Invitation Generated' : 'Invite New Team Member'}
            </h3>

            {!inviteLink ? (
              <form onSubmit={handleInvite}>
                <div style={{ marginBottom: '1rem' }}>
                  <label style={{ display: 'block', fontSize: '0.875rem', color: '#d1d5db', marginBottom: '0.5rem' }}>Email Address</label>
                  <input
                    type="email"
                    placeholder="member@example.com"
                    value={inviteEmail}
                    onChange={(e) => setInviteEmail(e.target.value)}
                    required
                    style={{ width: '100%', padding: '0.6rem 0.8rem', background: '#090d16', border: '1px solid #374151', borderRadius: '6px', color: '#fff', fontSize: '0.875rem' }}
                  />
                </div>
                <div style={{ marginBottom: '1.5rem' }}>
                  <label style={{ display: 'block', fontSize: '0.875rem', color: '#d1d5db', marginBottom: '0.5rem' }}>Role</label>
                  <select
                    value={inviteRole}
                    onChange={(e) => setInviteRole(e.target.value as any)}
                    style={{ width: '100%', padding: '0.6rem 0.8rem', background: '#090d16', border: '1px solid #374151', borderRadius: '6px', color: '#fff', fontSize: '0.875rem' }}
                  >
                    <option value="member">Member (Can call tools and manage own keys)</option>
                    <option value="admin">Admin (Can manage servers, users, and all keys)</option>
                  </select>
                </div>
                <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '0.5rem' }}>
                  <button
                    type="button"
                    onClick={() => setShowInviteModal(false)}
                    style={{ padding: '0.5rem 1rem', background: 'transparent', color: '#9ca3af', border: '1px solid #374151', borderRadius: '6px', cursor: 'pointer' }}
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    style={{ padding: '0.5rem 1rem', background: '#3b82f6', color: '#fff', border: 'none', borderRadius: '6px', fontWeight: 600, cursor: 'pointer' }}
                  >
                    Create Invite Link
                  </button>
                </div>
              </form>
            ) : (
              <div>
                <p style={{ color: '#9ca3af', fontSize: '0.875rem', margin: '0 0 1rem 0' }}>
                  Share this single-use link with the member. It expires in 72 hours:
                </p>
                <div style={{ background: '#090d16', border: '1px solid #374151', borderRadius: '6px', padding: '0.75rem', display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1.5rem' }}>
                  <code style={{ color: '#60a5fa', fontSize: '0.8rem', wordBreak: 'break-all' }}>{inviteLink}</code>
                  <button
                    onClick={copyInvite}
                    style={{ padding: '0.35rem 0.75rem', background: copied ? '#10b981' : '#3b82f6', color: '#fff', border: 'none', borderRadius: '4px', fontSize: '0.75rem', cursor: 'pointer', marginLeft: '0.5rem', whiteSpace: 'nowrap' }}
                  >
                    {copied ? 'Copied!' : 'Copy'}
                  </button>
                </div>
                <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
                  <button
                    onClick={() => setShowInviteModal(false)}
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
