package mcpserver

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// toolsListDigests pins the exact serialized tools/list result of each profile:
// names, descriptions, annotations, and the derived input and output schemas.
// A change to schema derivation or shaping that alters what a client sees must
// update these values on purpose.
var toolsListDigests = map[Profile]struct {
	tools  int
	bytes  int
	sha256 string
}{
	ProfileAll:      {tools: 26, bytes: 120732, sha256: "ce4ec9080c6557063de9ea53d7a0521172ef01c88815b239ce4592b1daf39b2a"},
	ProfileRuntime:  {tools: 3, bytes: 15363, sha256: "2a3e8f0a4effce0a65fba6347cd308422b46d1876d6f42b82fb496cf3e408b01"},
	ProfileCuration: {tools: 23, bytes: 105406, sha256: "06e16f0c279be16ff806d409fdb4ab1ff672cdd31b8f6af2eff8322f9c3db344"},
}

func serializeToolsList(t testing.TB, tools []*mcp.Tool) (int, string) {
	t.Helper()
	encoded, err := json.Marshal(&mcp.ListToolsResult{Tools: tools})
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(encoded)
	return len(encoded), hex.EncodeToString(sum[:])
}

func TestToolsListSerializationIsStablePerProfile(t *testing.T) {
	for profile, want := range toolsListDigests {
		t.Run(string(profile), func(t *testing.T) {
			tools := listProfileTools(t, profile).Tools
			size, digest := serializeToolsList(t, tools)
			if len(tools) != want.tools || size != want.bytes || digest != want.sha256 {
				t.Fatalf("tools/list for %s changed: tools=%d bytes=%d sha256=%s; pinned tools=%d bytes=%d sha256=%s", profile, len(tools), size, digest, want.tools, want.bytes, want.sha256)
			}
		})
	}
}

func TestProxyRuntimeToolsMatchHubRuntimeProfile(t *testing.T) {
	proxyTools, err := runtimeToolDefinitions()
	if err != nil {
		t.Fatal(err)
	}
	proxySize, proxyDigest := serializeToolsList(t, proxyTools)
	hubSize, hubDigest := serializeToolsList(t, listProfileTools(t, ProfileRuntime).Tools)
	if proxySize != hubSize || proxyDigest != hubDigest {
		t.Fatalf("proxy runtime tools (%d bytes, %s) differ from the runtime profile (%d bytes, %s)", proxySize, proxyDigest, hubSize, hubDigest)
	}
}

func TestToolsListUnchangedOnceEveryToolIsRegisteredInFull(t *testing.T) {
	for profile, want := range toolsListDigests {
		t.Run(string(profile), func(t *testing.T) {
			adapter, server, err := New(newMCPWorkspace(t), nil, profile)
			if err != nil {
				t.Fatal(err)
			}
			if err := materializeEveryTool(adapter.tools); err != nil {
				t.Fatalf("a tool of the %s profile cannot be registered in full: %v", profile, err)
			}
			client := mcp.NewClient(&mcp.Implementation{Name: "digest", Version: "1"}, nil)
			clientTransport, serverTransport := mcp.NewInMemoryTransports()
			serverSession, err := server.Connect(t.Context(), serverTransport, nil)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = serverSession.Close() })
			session, err := client.Connect(t.Context(), clientTransport, nil)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = session.Close() })
			listed, err := session.ListTools(t.Context(), nil)
			if err != nil {
				t.Fatal(err)
			}
			size, digest := serializeToolsList(t, listed.Tools)
			if len(listed.Tools) != want.tools || size != want.bytes || digest != want.sha256 {
				t.Fatalf("tools/list after full registration: tools=%d bytes=%d sha256=%s; pinned tools=%d bytes=%d sha256=%s", len(listed.Tools), size, digest, want.tools, want.bytes, want.sha256)
			}
		})
	}
}

// materializeEveryTool adds every listed tool as its typed form. It fails with
// the SDK's own error when a tool's schemas cannot be resolved, which would
// otherwise surface at the first call.
func materializeEveryTool(registry *toolRegistry) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("tool registration failed: %v", recovered)
		}
	}()
	for name := range registry.pending {
		registry.materialize(name)
	}
	return nil
}
