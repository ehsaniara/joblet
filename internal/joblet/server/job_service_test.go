package server

import (
	"testing"

	pb "github.com/ehsaniara/joblet-proto/v2/gen"
	"github.com/ehsaniara/joblet/internal/joblet/domain"
	"github.com/ehsaniara/joblet/pkg/logger"
)

// TestConvertToJobRequest_IgnoresClientJobTypeEnv guards the rule that the
// server never derives JobType from client-supplied environment variables:
// a client-set JOB_TYPE=runtime-build would otherwise skip the privilege drop
// and run as host root. RunJob must always produce standard-isolation jobs.
// A client must not be able to supply variables in joblet's reserved namespace:
// the in-namespace init trusts them to decide execution mode and isolation, and
// os/exec's last-duplicate-wins would let a client value override the trusted
// one. Reserved keys are rejected at the request boundary, in both the plain and
// secret environment maps.
func TestConvertToJobRequest_RejectsReservedEnv(t *testing.T) {
	s := &JobServiceServer{logger: logger.New().WithField("test", "true")}

	reserved := []struct {
		name string
		key  string
	}{
		{"mode flip", "JOBLET_MODE"},
		{"job type", "JOB_TYPE"},
		{"forwarded fs path", "JOB_FS_TMP_DIR"},
		{"runtime manager path", "RUNTIME_MANAGER_PATH"},
		{"network ready file", "NETWORK_READY_FILE"},
	}

	for _, r := range reserved {
		t.Run("plain/"+r.name, func(t *testing.T) {
			_, err := s.convertToJobRequest(&pb.RunJobRequest{
				Command:     "/bin/true",
				Environment: map[string]string{r.key: "x"},
			})
			if err == nil {
				t.Fatalf("reserved key %q accepted in Environment; want rejection", r.key)
			}
		})
		t.Run("secret/"+r.name, func(t *testing.T) {
			_, err := s.convertToJobRequest(&pb.RunJobRequest{
				Command:           "/bin/true",
				SecretEnvironment: map[string]string{r.key: "x"},
			})
			if err == nil {
				t.Fatalf("reserved key %q accepted in SecretEnvironment; want rejection", r.key)
			}
		})
	}

	// Legitimate client env is accepted, and jobs are always standard isolation.
	for _, name := range []string{"nil", "empty", "legit"} {
		t.Run("allowed/"+name, func(t *testing.T) {
			env := map[string]string{}
			switch name {
			case "nil":
				env = nil
			case "legit":
				env = map[string]string{"FOO": "bar", "MY_VAR": "value"}
			}
			got, err := s.convertToJobRequest(&pb.RunJobRequest{Command: "/bin/true", Environment: env})
			if err != nil {
				t.Fatalf("convertToJobRequest rejected legitimate env: %v", err)
			}
			if got.JobType != domain.JobTypeStandard {
				t.Fatalf("JobType = %q; want %q", got.JobType, domain.JobTypeStandard)
			}
		})
	}
}
