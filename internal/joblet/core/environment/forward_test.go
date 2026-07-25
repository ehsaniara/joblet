package environment

import (
	"strings"
	"testing"

	"github.com/ehsaniara/joblet/pkg/config"
)

func TestForwardRuntimeEnv(t *testing.T) {
	rt := &config.RuntimeConfig{
		BasePath:      "/opt/joblet/runtimes",
		AllowedMounts: []string{"/usr/bin", "/usr/sbin", "/etc/ssl"},
	}
	env := ForwardRuntimeEnv(rt)

	joined := strings.Join(env, "\n")
	if !strings.Contains(joined, "JOB_RT_BASE_PATH=/opt/joblet/runtimes") {
		t.Errorf("missing JOB_RT_BASE_PATH in %v", env)
	}
	if !strings.Contains(joined, "JOB_RT_ALLOWED_MOUNTS=/usr/bin:/usr/sbin:/etc/ssl") {
		t.Errorf("AllowedMounts not path-list joined in %v", env)
	}

	// Empty AllowedMounts must not emit the var (init keeps built-in default).
	env2 := ForwardRuntimeEnv(&config.RuntimeConfig{BasePath: "/x"})
	for _, kv := range env2 {
		if strings.HasPrefix(kv, "JOB_RT_ALLOWED_MOUNTS=") {
			t.Errorf("empty AllowedMounts should not be forwarded, got %q", kv)
		}
	}
}
