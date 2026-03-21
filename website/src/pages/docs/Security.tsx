import DocsLayout, {
  DocHeading,
  CodeBlock,
  InfoBox,
  type TocItem,
} from "@/components/DocsLayout";
import { Shield, Lock, KeyRound, UserCheck, FileKey, Network } from "lucide-react";

const toc: TocItem[] = [
  { id: "overview", text: "Overview", level: 2 },
  { id: "authentication", text: "Authentication", level: 2 },
  { id: "tls", text: "TLS Encryption", level: 2 },
  { id: "tls-generate", text: "Generate Certificates", level: 3 },
  { id: "tls-config", text: "TLS Configuration", level: 3 },
  { id: "tls-mutual", text: "Mutual TLS (mTLS)", level: 3 },
  { id: "acl", text: "Access Control Lists", level: 2 },
  { id: "acl-config", text: "ACL Configuration", level: 3 },
  { id: "acl-rules", text: "ACL Rule Syntax", level: 3 },
  { id: "acl-commands", text: "ACL Commands", level: 3 },
  { id: "network", text: "Network Security", level: 2 },
  { id: "best-practices", text: "Best Practices", level: 2 },
];

export default function Security() {
  return (
    <DocsLayout toc={toc}>
      {/* Hero */}
      <div className="mb-10">
        <div className="flex items-center gap-2 text-blue-400 text-sm font-medium mb-2">
          <Shield className="w-4 h-4" />
          Operations
        </div>
        <h1 className="text-4xl font-extrabold text-white tracking-tight mb-4">
          Security Guide
        </h1>
        <p className="text-lg text-slate-400 leading-relaxed max-w-2xl">
          Secure your CacheStorm deployment with TLS encryption, access control lists,
          and authentication. This guide covers all security features and best practices.
        </p>
      </div>

      {/* ── Overview ─────────────────────────────────────────── */}
      <DocHeading id="overview" level={2}>
        Overview
      </DocHeading>

      <p className="mb-4 text-slate-400">
        CacheStorm provides multiple layers of security:
      </p>

      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 mb-6">
        {[
          { icon: <Lock className="w-5 h-5 text-blue-400" />, title: "Authentication", desc: "Password-based auth with AUTH command" },
          { icon: <FileKey className="w-5 h-5 text-emerald-400" />, title: "TLS/SSL", desc: "End-to-end encryption for all connections" },
          { icon: <UserCheck className="w-5 h-5 text-amber-400" />, title: "ACL", desc: "Fine-grained per-user command and key permissions" },
          { icon: <Network className="w-5 h-5 text-purple-400" />, title: "Network", desc: "Bind address restrictions and firewall rules" },
        ].map((item) => (
          <div
            key={item.title}
            className="flex items-start gap-3 p-4 rounded-xl border border-slate-800 bg-slate-900/50"
          >
            {item.icon}
            <div>
              <p className="text-sm font-semibold text-white">{item.title}</p>
              <p className="text-xs text-slate-500 mt-0.5">{item.desc}</p>
            </div>
          </div>
        ))}
      </div>

      <InfoBox type="warning">
        Never expose CacheStorm directly to the public internet without proper authentication
        and TLS encryption. Use a firewall or VPN for production deployments.
      </InfoBox>

      {/* ── Authentication ───────────────────────────────────── */}
      <DocHeading id="authentication" level={2}>
        <KeyRound className="w-5 h-5 text-blue-400" />
        Authentication
      </DocHeading>

      <p className="mb-4 text-slate-400">
        Simple password-based authentication using the <code className="text-xs bg-slate-800 px-1 py-0.5 rounded">requirepass</code> directive.
      </p>

      <CodeBlock
        language="yaml"
        title="cachestorm.yaml"
        code={`security:
  # Use an environment variable for the password
  password: "\${CACHESTORM_PASSWORD}"`}
      />

      <CodeBlock
        language="bash"
        title="Connecting with authentication"
        code={`# Set the password
export CACHESTORM_PASSWORD="your-strong-password-here"

# Connect with redis-cli
redis-cli -p 6380 -a "your-strong-password-here"

# Or authenticate after connecting
redis-cli -p 6380
127.0.0.1:6380> AUTH your-strong-password-here
OK

# With ACL users (username + password)
127.0.0.1:6380> AUTH admin my-admin-password
OK`}
      />

      <InfoBox type="tip">
        Always use environment variables or secrets management for passwords.
        Never commit passwords to version control.
      </InfoBox>

      {/* ── TLS ──────────────────────────────────────────────── */}
      <DocHeading id="tls" level={2}>
        <Lock className="w-5 h-5 text-blue-400" />
        TLS Encryption
      </DocHeading>

      <p className="mb-4 text-slate-400">
        TLS encrypts all traffic between clients and the server, preventing eavesdropping
        and man-in-the-middle attacks.
      </p>

      <DocHeading id="tls-generate" level={3}>
        Generate Certificates
      </DocHeading>

      <CodeBlock
        language="bash"
        title="Generate self-signed certificates (development)"
        code={`# Create a directory for certificates
mkdir -p /etc/cachestorm/tls && cd /etc/cachestorm/tls

# Generate CA key and certificate
openssl genrsa -out ca.key 4096
openssl req -new -x509 -days 365 -key ca.key \\
  -out ca.crt -subj "/CN=CacheStorm CA"

# Generate server key and certificate
openssl genrsa -out server.key 2048
openssl req -new -key server.key \\
  -out server.csr -subj "/CN=cachestorm-server"

# Sign the server certificate with our CA
openssl x509 -req -days 365 \\
  -in server.csr -CA ca.crt -CAkey ca.key \\
  -CAcreateserial -out server.crt

# Clean up CSR
rm server.csr`}
      />

      <InfoBox type="info">
        For production, use certificates from a trusted CA (Let's Encrypt, DigiCert, etc.)
        or your organization's internal PKI.
      </InfoBox>

      <DocHeading id="tls-config" level={3}>
        TLS Configuration
      </DocHeading>

      <CodeBlock
        language="yaml"
        title="cachestorm.yaml"
        code={`security:
  tls:
    enabled: true
    cert_file: "/etc/cachestorm/tls/server.crt"
    key_file: "/etc/cachestorm/tls/server.key"
    # Minimum TLS version (recommended: 1.2 or 1.3)
    min_version: "1.2"`}
      />

      <CodeBlock
        language="bash"
        title="Connect with TLS"
        code={`# Using redis-cli with TLS
redis-cli -p 6380 --tls \\
  --cert /etc/cachestorm/tls/client.crt \\
  --key /etc/cachestorm/tls/client.key \\
  --cacert /etc/cachestorm/tls/ca.crt

# Test TLS connection
openssl s_client -connect localhost:6380 \\
  -CAfile /etc/cachestorm/tls/ca.crt`}
      />

      <DocHeading id="tls-mutual" level={3}>
        Mutual TLS (mTLS)
      </DocHeading>

      <p className="mb-4 text-slate-400">
        Mutual TLS requires both the server and client to present certificates,
        providing strong two-way authentication.
      </p>

      <CodeBlock
        language="yaml"
        title="cachestorm.yaml (mTLS)"
        code={`security:
  tls:
    enabled: true
    cert_file: "/etc/cachestorm/tls/server.crt"
    key_file: "/etc/cachestorm/tls/server.key"
    ca_file: "/etc/cachestorm/tls/ca.crt"  # Enables client cert verification
    client_auth: "require"  # require | request | none`}
      />

      <CodeBlock
        language="bash"
        title="Generate client certificate"
        code={`# Generate client key and certificate
openssl genrsa -out client.key 2048
openssl req -new -key client.key \\
  -out client.csr -subj "/CN=cachestorm-client"
openssl x509 -req -days 365 \\
  -in client.csr -CA ca.crt -CAkey ca.key \\
  -CAcreateserial -out client.crt
rm client.csr`}
      />

      {/* ── ACL ──────────────────────────────────────────────── */}
      <DocHeading id="acl" level={2}>
        <UserCheck className="w-5 h-5 text-blue-400" />
        Access Control Lists (ACL)
      </DocHeading>

      <p className="mb-4 text-slate-400">
        ACLs provide fine-grained access control, allowing you to define per-user permissions
        for commands, keys, and channels.
      </p>

      <DocHeading id="acl-config" level={3}>
        ACL Configuration
      </DocHeading>

      <CodeBlock
        language="yaml"
        title="cachestorm.yaml"
        code={`security:
  acl:
    enabled: true
    file: "/etc/cachestorm/acl.conf"
    # Default user password (when not using ACL file)
    default_user_password: "\${CACHESTORM_DEFAULT_PASSWORD}"`}
      />

      <CodeBlock
        language="bash"
        title="/etc/cachestorm/acl.conf"
        code={`# Default user (full access)
user default on >defaultpassword ~* &* +@all

# Admin user (full access)
user admin on >admin-secret-password ~* &* +@all

# Read-only user
user reader on >reader-password ~* &* +@read -@write -@admin -@dangerous

# Application user (limited to specific key patterns)
user app on >app-password ~app:* ~cache:* &* +@read +@write +@connection -@admin

# Analytics user (read-only, specific keys)
user analytics on >analytics-password ~metrics:* ~stats:* &* +get +mget +hgetall +info

# Pub/Sub only user
user pubsub_user on >pubsub-password &events:* +subscribe +publish +psubscribe`}
      />

      <DocHeading id="acl-rules" level={3}>
        ACL Rule Syntax
      </DocHeading>

      <div className="my-4 rounded-xl border border-slate-800 overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-slate-800 text-left text-slate-400">
                <th className="px-4 py-2 font-medium">Rule</th>
                <th className="px-4 py-2 font-medium">Description</th>
              </tr>
            </thead>
            <tbody className="text-slate-300">
              {[
                ["on / off", "Enable or disable the user"],
                [">password", "Add a password for the user"],
                ["~pattern", "Allow access to keys matching the glob pattern"],
                ["&pattern", "Allow access to Pub/Sub channels matching the pattern"],
                ["+command", "Allow a specific command"],
                ["-command", "Deny a specific command"],
                ["+@category", "Allow all commands in a category"],
                ["-@category", "Deny all commands in a category"],
                ["allcommands / +@all", "Allow all commands"],
                ["allkeys / ~*", "Allow access to all keys"],
                ["resetkeys", "Reset all key patterns"],
                ["nopass", "Allow connecting without a password"],
              ].map(([rule, desc], i, arr) => (
                <tr key={rule} className={i < arr.length - 1 ? "border-b border-slate-800/60" : ""}>
                  <td className="px-4 py-2 font-mono text-xs text-blue-300 whitespace-nowrap">{rule}</td>
                  <td className="px-4 py-2 text-slate-400">{desc}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      <p className="mb-3 text-slate-400">Available command categories:</p>

      <div className="flex flex-wrap gap-2 mb-6">
        {[
          "@read", "@write", "@set", "@hash", "@list", "@sortedset",
          "@string", "@stream", "@pubsub", "@admin", "@dangerous",
          "@connection", "@server", "@scripting", "@fast", "@slow",
        ].map((cat) => (
          <span
            key={cat}
            className="text-xs font-mono px-2 py-1 rounded-md bg-slate-800 text-slate-300 border border-slate-700"
          >
            {cat}
          </span>
        ))}
      </div>

      <DocHeading id="acl-commands" level={3}>
        ACL Commands
      </DocHeading>

      <CodeBlock
        language="bash"
        title="Runtime ACL management"
        code={`# List all users
ACL LIST

# Get current user info
ACL WHOAMI

# Create a new user at runtime
ACL SETUSER newuser on >password ~cache:* +get +set +del

# Get user details
ACL GETUSER newuser

# Delete a user
ACL DELUSER newuser

# List available categories
ACL CAT

# List commands in a category
ACL CAT read

# Save ACL changes to file
ACL SAVE

# Reload ACL from file
ACL LOAD`}
      />

      {/* ── Network Security ─────────────────────────────────── */}
      <DocHeading id="network" level={2}>
        <Network className="w-5 h-5 text-blue-400" />
        Network Security
      </DocHeading>

      <p className="mb-4 text-slate-400">
        Restrict which network interfaces CacheStorm listens on and use firewall rules to
        limit access.
      </p>

      <CodeBlock
        language="yaml"
        title="Network configuration"
        code={`server:
  # Listen only on localhost (most secure for single-machine setups)
  bind: "127.0.0.1"

  # Or listen on a specific internal interface
  # bind: "10.0.1.10"

  # Or listen on all interfaces (requires authentication!)
  # bind: "0.0.0.0"`}
      />

      <CodeBlock
        language="bash"
        title="iptables firewall rules"
        code={`# Allow CacheStorm access only from application servers
iptables -A INPUT -p tcp --dport 6380 -s 10.0.1.0/24 -j ACCEPT
iptables -A INPUT -p tcp --dport 6380 -j DROP

# Allow HTTP API only from monitoring
iptables -A INPUT -p tcp --dport 7280 -s 10.0.2.0/24 -j ACCEPT
iptables -A INPUT -p tcp --dport 7280 -j DROP`}
      />

      {/* ── Best Practices ───────────────────────────────────── */}
      <DocHeading id="best-practices" level={2}>
        Best Practices
      </DocHeading>

      <div className="space-y-3 mb-6">
        {[
          {
            title: "Enable TLS in production",
            desc: "Always encrypt connections to prevent data leakage and MITM attacks.",
          },
          {
            title: "Use strong passwords",
            desc: "Minimum 32 characters with mixed case, numbers, and symbols. Rotate regularly.",
          },
          {
            title: "Apply least-privilege ACLs",
            desc: "Give each application user only the commands and key patterns they need.",
          },
          {
            title: "Bind to specific interfaces",
            desc: "Never bind to 0.0.0.0 without authentication. Prefer localhost or internal IPs.",
          },
          {
            title: "Use firewall rules",
            desc: "Restrict port access to known application servers and monitoring systems.",
          },
          {
            title: "Disable dangerous commands",
            desc: "Restrict FLUSHALL, FLUSHDB, CONFIG, DEBUG, and KEYS via ACLs for non-admin users.",
          },
          {
            title: "Audit and monitor",
            desc: "Enable slow log, monitor AUTH failures, and alert on suspicious activity.",
          },
          {
            title: "Keep CacheStorm updated",
            desc: "Apply security patches promptly. Subscribe to release notifications.",
          },
        ].map((item) => (
          <div
            key={item.title}
            className="flex items-start gap-3 p-3 rounded-lg border border-slate-800 bg-slate-900/30"
          >
            <div className="w-1.5 h-1.5 rounded-full bg-blue-400 mt-2 shrink-0" />
            <div>
              <p className="text-sm font-medium text-white">{item.title}</p>
              <p className="text-xs text-slate-500 mt-0.5">{item.desc}</p>
            </div>
          </div>
        ))}
      </div>
    </DocsLayout>
  );
}
