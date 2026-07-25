package server

import (
	"testing"

	pb "github.com/ehsaniara/joblet-proto/v2/gen"
	"github.com/ehsaniara/joblet/internal/joblet/domain"
	"github.com/ehsaniara/joblet/pkg/logger"
)

// TestConvertToJobRequest_IgnoresClientJobTypeEnv is a regression test for
// the runtime-build privilege-escalation bug (issue #258). The server must
// never derive the JobType from client-supplied environment variables;
// RunJob always produces standard-isolation jobs.
func TestConvertToJobRequest_IgnoresClientJobTypeEnv(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
	}{
		{"runtime-build", map[string]string{"JOB_TYPE": "runtime-build"}},
		{"mixed case", map[string]string{"JOB_TYPE": "Runtime-Build"}},
		{"runtime-build with siblings", map[string]string{"JOB_TYPE": "runtime-build", "FOO": "bar"}},
		{"empty env", map[string]string{}},
		{"nil env", nil},
	}

	s := &JobServiceServer{logger: logger.New().WithField("test", "true")}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := &pb.RunJobRequest{
				Command:     "/bin/true",
				Environment: tc.env,
			}

			got, err := s.convertToJobRequest(req)
			if err != nil {
				t.Fatalf("convertToJobRequest returned error: %v", err)
			}
			if got.JobType != domain.JobTypeStandard {
				t.Fatalf("JobType = %q; want %q (client must not influence isolation mode)",
					got.JobType, domain.JobTypeStandard)
			}
		})
	}
}
