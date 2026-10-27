import React, { useState, useRef, useEffect } from 'react';
import { ChatMessage, ToolCallChip } from '../types';

export const ChatTab: React.FC = () => {
  const [messages, setMessages] = useState<ChatMessage[]>([
    {
      id: 'welcome',
      role: 'assistant',
      content: 'Hello! I am Urumi, your in-cluster store operations assistant. I execute tools through the MCP Gateway as your authenticated identity. You can ask me to list recent store orders, create products, look up customer orders, or check current timestamps across timezones.',
    },
  ]);
  const [input, setInput] = useState('');
  const [loading, setLoading] = useState(false);
  const [conversationID, setConversationID] = useState<string>('');
  const messagesEndRef = useRef<HTMLDivElement>(null);

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  };

  useEffect(() => {
    scrollToBottom();
  }, [messages, loading]);

  const handleSend = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!input.trim() || loading) return;

    const userText = input.trim();
    setInput('');
    const userMsg: ChatMessage = {
      id: 'user-' + Date.now(),
      role: 'user',
      content: userText,
    };

    const assistantMsgId = 'assistant-' + Date.now();
    const initialAssistantMsg: ChatMessage = {
      id: assistantMsgId,
      role: 'assistant',
      content: '',
      toolCalls: [],
    };

    setMessages((prev) => [...prev, userMsg, initialAssistantMsg]);
    setLoading(true);

    try {
      const response = await fetch('/api/chat', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          conversation_id: conversationID,
          message: userText,
        }),
      });

      if (!response.ok || !response.body) {
        throw new Error('Failed to start chat stream');
      }

      const reader = response.body.getReader();
      const decoder = new TextDecoder();
      let buffer = '';

      while (true) {
        const { value, done } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split('\n\n');
        buffer = lines.pop() || '';

        for (const block of lines) {
          if (!block.trim()) continue;

          let event = 'message';
          let dataStr = '';

          const blockLines = block.split('\n');
          for (const line of blockLines) {
            if (line.startsWith('event: ')) {
              event = line.slice(7).trim();
            } else if (line.startsWith('data: ')) {
              dataStr = line.slice(6).trim();
            }
          }

          if (event === 'token') {
            try {
              const text = JSON.parse(dataStr);
              setMessages((prev) =>
                prev.map((m) =>
                  m.id === assistantMsgId
                    ? { ...m, content: m.content + text }
                    : m
                )
              );
            } catch {
              setMessages((prev) =>
                prev.map((m) =>
                  m.id === assistantMsgId ? { ...m, content: m.content + dataStr } : m
                )
              );
            }
          } else if (event === 'tool_call_started') {
            try {
              const tc: ToolCallChip = JSON.parse(dataStr);
              setMessages((prev) =>
                prev.map((m) => {
                  if (m.id !== assistantMsgId) return m;
                  const existing = m.toolCalls || [];
                  return { ...m, toolCalls: [...existing, tc] };
                })
              );
            } catch (err) {
              console.error(err);
            }
          } else if (event === 'tool_call_finished') {
            try {
              const finished: ToolCallChip = JSON.parse(dataStr);
              setMessages((prev) =>
                prev.map((m) => {
                  if (m.id !== assistantMsgId) return m;
                  const updated = (m.toolCalls || []).map((tc) =>
                    tc.id === finished.id ? { ...tc, ...finished } : tc
                  );
                  return { ...m, toolCalls: updated };
                })
              );
            } catch (err) {
              console.error(err);
            }
          } else if (event === 'done') {
            try {
              const d = JSON.parse(dataStr);
              if (d.conversation_id) setConversationID(d.conversation_id);
            } catch {}
          }
        }
      }
    } catch (err: any) {
      setMessages((prev) =>
        prev.map((m) =>
          m.id === assistantMsgId
            ? { ...m, content: `Error: ${err.message || 'Failed to complete message'}` }
            : m
        )
      );
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: 'calc(100vh - 160px)', background: '#111827', border: '1px solid #1f2937', borderRadius: '12px', overflow: 'hidden' }}>
      <div style={{ padding: '0.875rem 1.25rem', borderBottom: '1px solid #1f2937', background: '#090d16', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
          <span style={{ fontSize: '1rem', fontWeight: 600 }}>Urumi Assistant</span>
          <span style={{ fontSize: '0.75rem', background: '#1e3a8a', color: '#bfdbfe', padding: '0.15rem 0.5rem', borderRadius: '10px' }}>
            In-Process Tool Loop
          </span>
        </div>
        <span style={{ fontSize: '0.75rem', color: '#9ca3af' }}>
          Channel: <code style={{ color: '#60a5fa' }}>chat</code>
        </span>
      </div>

      <div style={{ flex: 1, overflowY: 'auto', padding: '1.25rem', display: 'flex', flexDirection: 'column', gap: '1rem' }}>
        {messages.map((m) => (
          <div
            key={m.id}
            style={{
              display: 'flex',
              flexDirection: 'column',
              alignItems: m.role === 'user' ? 'flex-end' : 'flex-start',
            }}
          >
            <div
              style={{
                maxWidth: '80%',
                padding: '0.875rem 1.1rem',
                borderRadius: '10px',
                background: m.role === 'user' ? '#2563eb' : '#1f2937',
                color: '#f3f4f6',
                fontSize: '0.9rem',
                lineHeight: 1.5,
                whiteSpace: 'pre-wrap',
                boxShadow: '0 2px 4px rgba(0,0,0,0.1)',
              }}
            >
              {m.content || (loading && m.role === 'assistant' ? 'Thinking...' : '')}
            </div>

            {m.toolCalls && m.toolCalls.length > 0 && (
              <div style={{ marginTop: '0.5rem', display: 'flex', flexDirection: 'column', gap: '0.4rem', width: '100%', maxWidth: '80%' }}>
                {m.toolCalls.map((tc) => (
                  <ToolChip key={tc.id} tool={tc} />
                ))}
              </div>
            )}
          </div>
        ))}
        <div ref={messagesEndRef} />
      </div>

      <form onSubmit={handleSend} style={{ padding: '1rem', borderTop: '1px solid #1f2937', background: '#090d16', display: 'flex', gap: '0.75rem' }}>
        <input
          value={input}
          onChange={(e) => setInput(e.target.value)}
          placeholder="Ask: 'Show recent orders and current time' or 'Create product Demo Headphones'..."
          disabled={loading}
          style={{ flex: 1, padding: '0.75rem 1rem', background: '#111827', border: '1px solid #374151', borderRadius: '8px', color: '#fff', fontSize: '0.875rem', outline: 'none' }}
        />
        <button
          type="submit"
          disabled={loading || !input.trim()}
          style={{ padding: '0.75rem 1.5rem', background: '#3b82f6', color: '#fff', border: 'none', borderRadius: '8px', fontWeight: 600, fontSize: '0.875rem', cursor: loading ? 'not-allowed' : 'pointer', opacity: loading || !input.trim() ? 0.6 : 1 }}
        >
          {loading ? 'Sending...' : 'Send'}
        </button>
      </form>
    </div>
  );
};

const ToolChip: React.FC<{ tool: ToolCallChip }> = ({ tool }) => {
  const [open, setOpen] = useState(false);
  const isFinished = !!tool.outcome;
  const isSuccess = tool.outcome === 'success';

  return (
    <div style={{ background: '#090d16', border: `1px solid ${isFinished ? (isSuccess ? 'rgba(16,185,129,0.3)' : 'rgba(239,68,68,0.3)') : '#374151'}`, borderRadius: '6px', padding: '0.4rem 0.6rem', fontSize: '0.75rem' }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', cursor: 'pointer' }} onClick={() => setOpen(!open)}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
          <span style={{ color: isFinished ? (isSuccess ? '#10b981' : '#ef4444') : '#f59e0b' }}>
            {isFinished ? (isSuccess ? '✓' : '✗') : '⟳'}
          </span>
          <code style={{ color: '#60a5fa', fontWeight: 600 }}>{tool.name}</code>
          {tool.duration !== undefined && (
            <span style={{ color: '#9ca3af' }}>({tool.duration} ms)</span>
          )}
        </div>
        <span style={{ color: '#9ca3af', fontSize: '0.7rem' }}>{open ? '▲ Collapse' : '▼ View Result'}</span>
      </div>

      {open && (
        <div style={{ marginTop: '0.5rem', borderTop: '1px solid #1f2937', paddingTop: '0.5rem' }}>
          {tool.args && (
            <div style={{ marginBottom: '0.25rem' }}>
              <span style={{ color: '#9ca3af' }}>Args: </span>
              <code style={{ color: '#d1d5db' }}>{tool.args}</code>
            </div>
          )}
          {tool.preview && (
            <div>
              <span style={{ color: '#9ca3af' }}>Result: </span>
              <pre style={{ margin: '0.25rem 0 0 0', padding: '0.4rem', background: '#111827', borderRadius: '4px', overflowX: 'auto', color: '#e5e7eb', fontSize: '0.7rem' }}>
                {tool.preview}
              </pre>
            </div>
          )}
        </div>
      )}
    </div>
  );
};
