import React, { useState } from 'react';

export const ConnectTab: React.FC = () => {
  const [copiedSection, setCopiedSection] = useState<string | null>(null);

  const copySnippet = (id: string, text: string) => {
    navigator.clipboard.writeText(text);
    setCopiedSection(id);
    setTimeout(() => setCopiedSection(null), 2000);
  };

  const claudeSnippet = `claude mcp add --transport http urumi http://app.localtest.me/mcp --header "Authorization: Bearer <YOUR_API_KEY>"`;
  const cursorSnippet = `{
  "mcpServers": {
    "urumi": {
      "url": "http://app.localtest.me/mcp",
      "headers": {
        "Authorization": "Bearer <YOUR_API_KEY>"
      }
    }
  }
}`;
  const inspectorSnippet = `npx @modelcontextprotocol/inspector http://app.localtest.me/mcp --header "Authorization: Bearer <YOUR_API_KEY>"`;

  return (
    <div>
      <div style={{ marginBottom: '1.5rem' }}>
        <h2 style={{ margin: 0, fontSize: '1.5rem', fontWeight: 600 }}>Connect External MCP Clients</h2>
        <p style={{ margin: '0.25rem 0 0 0', color: '#9ca3af', fontSize: '0.875rem' }}>
          Connect your favorite AI editors and agents to the unified gateway endpoint. Replace <code style={{ color: '#60a5fa' }}>&lt;YOUR_API_KEY&gt;</code> with an active key from the <strong>API Keys</strong> tab.
        </p>
      </div>

      <div style={{ display: 'grid', gap: '1.5rem' }}>
        {/* Claude Code */}
        <div style={{ background: '#111827', border: '1px solid #1f2937', borderRadius: '10px', padding: '1.5rem' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '0.75rem' }}>
            <h3 style={{ margin: 0, fontSize: '1.1rem', fontWeight: 600 }}>Claude Code CLI</h3>
            <button
              onClick={() => copySnippet('claude', claudeSnippet)}
              style={{ padding: '0.35rem 0.75rem', background: copiedSection === 'claude' ? '#10b981' : '#1f2937', color: '#fff', border: 'none', borderRadius: '4px', fontSize: '0.75rem', cursor: 'pointer' }}
            >
              {copiedSection === 'claude' ? 'Copied!' : 'Copy Command'}
            </button>
          </div>
          <p style={{ color: '#9ca3af', fontSize: '0.8rem', margin: '0 0 0.75rem 0' }}>
            Run directly in your terminal to register Urumi as an HTTP MCP server:
          </p>
          <pre style={{ margin: 0, padding: '0.75rem', background: '#090d16', borderRadius: '6px', color: '#60a5fa', fontSize: '0.8rem', overflowX: 'auto', fontFamily: 'monospace' }}>
            {claudeSnippet}
          </pre>
        </div>

        {/* Cursor */}
        <div style={{ background: '#111827', border: '1px solid #1f2937', borderRadius: '10px', padding: '1.5rem' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '0.75rem' }}>
            <h3 style={{ margin: 0, fontSize: '1.1rem', fontWeight: 600 }}>Cursor Editor</h3>
            <button
              onClick={() => copySnippet('cursor', cursorSnippet)}
              style={{ padding: '0.35rem 0.75rem', background: copiedSection === 'cursor' ? '#10b981' : '#1f2937', color: '#fff', border: 'none', borderRadius: '4px', fontSize: '0.75rem', cursor: 'pointer' }}
            >
              {copiedSection === 'cursor' ? 'Copied!' : 'Copy JSON'}
            </button>
          </div>
          <p style={{ color: '#9ca3af', fontSize: '0.8rem', margin: '0 0 0.75rem 0' }}>
            Paste into your project's <code style={{ color: '#d1d5db' }}>.cursor/mcp.json</code>:
          </p>
          <pre style={{ margin: 0, padding: '0.75rem', background: '#090d16', borderRadius: '6px', color: '#34d399', fontSize: '0.8rem', overflowX: 'auto', fontFamily: 'monospace' }}>
            {cursorSnippet}
          </pre>
        </div>

        {/* MCP Inspector */}
        <div style={{ background: '#111827', border: '1px solid #1f2937', borderRadius: '10px', padding: '1.5rem' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '0.75rem' }}>
            <h3 style={{ margin: 0, fontSize: '1.1rem', fontWeight: 600 }}>Official MCP Inspector</h3>
            <button
              onClick={() => copySnippet('inspector', inspectorSnippet)}
              style={{ padding: '0.35rem 0.75rem', background: copiedSection === 'inspector' ? '#10b981' : '#1f2937', color: '#fff', border: 'none', borderRadius: '4px', fontSize: '0.75rem', cursor: 'pointer' }}
            >
              {copiedSection === 'inspector' ? 'Copied!' : 'Copy Command'}
            </button>
          </div>
          <p style={{ color: '#9ca3af', fontSize: '0.8rem', margin: '0 0 0.75rem 0' }}>
            Inspect tools and interactively test live calls through the official Model Context Protocol Inspector:
          </p>
          <pre style={{ margin: 0, padding: '0.75rem', background: '#090d16', borderRadius: '6px', color: '#f59e0b', fontSize: '0.8rem', overflowX: 'auto', fontFamily: 'monospace' }}>
            {inspectorSnippet}
          </pre>
        </div>
      </div>
    </div>
  );
};
