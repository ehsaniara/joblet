package execution

import (
	"strings"
	"testing"

	"github.com/ehsaniara/joblet/internal/joblet/domain"
	"github.com/ehsaniara/joblet/pkg/logger"
	"github.com/ehsaniara/joblet/pkg/platform"
)

// Defense in depth for the reserved-namespace rule: even if a job somehow
// carried a reserved key in its client environment, the trusted control block
// is appended last so it wins the os/exec last-duplicate race.
func TestBuildEnvironment_TrustedControlVarsWin(t *testing.T) {
	es := &EnvironmentService{
		platform: platform.NewPlatform(),
		logger:   logger.New(),
	}
	job := &domain.Job{
		Uuid:              "job-123",
		Command:           "/bin/true",
		Environment:       map[string]string{"JOBLET_MODE": "server"},
		SecretEnvironment: map[string]string{"JOB_ID": "attacker"},
	}

	env := es.BuildEnvironment(job, "execute")

	// os/exec keeps the LAST occurrence of a key, so assert the last wins.
	var lastMode, lastJobID string
	for _, kv := range env {
		if strings.HasPrefix(kv, "JOBLET_MODE=") {
			lastMode = kv
		}
		if strings.HasPrefix(kv, "JOB_ID=") {
			lastJobID = kv
		}
	}
	if lastMode != "JOBLET_MODE=init" {
		t.Errorf("last JOBLET_MODE = %q; want JOBLET_MODE=init (trusted must win)", lastMode)
	}
	if lastJobID != "JOB_ID=job-123" {
		t.Errorf("last JOB_ID = %q; want JOB_ID=job-123 (trusted must win)", lastJobID)
	}
}
