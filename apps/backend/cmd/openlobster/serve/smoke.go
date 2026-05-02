package serve

import (
	"encoding/json"
	"log"
	"strings"

	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/neirth/openlobster/internal/infrastructure/logging"
)

// probeConfiguredPlugin runs lightweight post-configure liveness and sanity
// checks against a plugin that has already been configured with real user
// configuration.  It is inspired by the utils/validation_layer error-path smoke
// — calling exported functions with nil input to verify the plugin subprocess
// does not crash or hang.
//
// For functions known to trigger external API calls with nil/default input
// (e.g. ai:chat, audio:tts, memory:store/retrieve/query), the probe is
// skipped to avoid side effects.  The validation layer CLI tool should be used
// for full functional smoke testing.
func probeConfiguredPlugin(p ports.PluginPort, cfg map[string]interface{}) {
	switch p.Type() {
	case "secrets":
		probeSecretsPlugin(p, cfg)
	case "memory":
		probeMemoryPlugin(p, cfg)
	case "messaging":
		probeMessagingPlugin(p)
	case "ai":
		// No safe nil-input probe for AI — chat sends real requests.
		// configure() success is sufficient liveness proof.
	case "audio":
		// No safe nil-input probe for audio — tts/stt cost money.
		logging.Debugf("plugins: audio liveness assumed from configure success")
	}
}

func probeSecretsPlugin(p ports.PluginPort, cfg map[string]interface{}) {
	// nil-input liveness: get/set/delete return early on missing required
	// fields without any external call.
	for _, fn := range []string{"get", "set", "delete"} {
		if !hasFn(p, fn) {
			continue
		}
		_, err := p.Call(fn, nil)
		if err != nil {
			if isTransportCrash(err) {
				log.Printf("plugins: secrets/%s CRASHED on nil-input probe: %v", fn, err)
				return
			}
		}
	}

	// Functional probe: call 'get' with a non-existent key using real config
	// to verify the secrets backend is reachable.  404 or connection error
	// from the backend is expected and non-fatal — we only care about
	// transport crashes.
	probe, _ := json.Marshal(map[string]interface{}{
		"config": cfg,
		"key":    "__openlobster_liveness_probe__",
	})
	_, err := p.Call("get", probe)
	if err != nil {
		if isTransportCrash(err) {
			log.Printf("plugins: secrets backend probe CRASHED: %v", err)
		} else {
			log.Printf("plugins: secrets backend connectivity check: %v", err)
		}
		return
	}
	logging.Debugf("plugins: secrets backend connectivity OK")
}

func probeMemoryPlugin(p ports.PluginPort, cfg map[string]interface{}) {
	// invalidate_cache is a no-op (no database connection) — safe for
	// liveness probe even with neo4j/neo4rs panicking on real connections.
	if hasFn(p, "invalidate_cache") {
		if _, err := p.Call("store", []byte(`{"op":"invalidate_cache","user_id":"__liveness__"}`)); err != nil {
			log.Printf("plugins: memory backend liveness probe: %v", err)
		}
	}

	// For file-based memory, a real query probe is safe.
	shortID := ""
	id := p.ID()
	if parts := strings.SplitN(id, ":", 2); len(parts) == 2 && parts[0] == "memory" {
		shortID = parts[1]
	}
	isFile := shortID == "file" || shortID == "gml"

	if isFile && hasFn(p, "query") {
		probe, _ := json.Marshal(map[string]interface{}{
			"config": cfg,
			"filter": map[string]interface{}{},
		})
		if _, err := p.Call("query", probe); err != nil {
			if isTransportCrash(err) {
				log.Printf("plugins: memory backend probe CRASHED: %v", err)
			} else {
				log.Printf("plugins: memory backend connectivity check: %v", err)
			}
			return
		}
		logging.Debugf("plugins: memory backend connectivity OK")
	}
}

func probeMessagingPlugin(p ports.PluginPort) {
	safeFns := []string{
		"resolve_channel_id", "capabilities", "inbound_mode",
		"send", "send_voice", "typing", "speaking",
	}
	for _, fn := range safeFns {
		if !hasFn(p, fn) {
			continue
		}
		_, err := p.Call(fn, nil)
		if err != nil {
			if isTransportCrash(err) {
				log.Printf("plugins: messaging/%s CRASHED on nil-input probe: %v", fn, err)
				return
			}
		}
	}
}

func hasFn(p ports.PluginPort, fn string) bool {
	if introspector, ok := p.(ports.PluginFunctionIntrospectionPort); ok {
		return introspector.HasFunction(fn)
	}
	return true // can't check, assume available
}

func isTransportCrash(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "broken pipe") || strings.Contains(s, "eof") || strings.Contains(s, "timeout")
}
