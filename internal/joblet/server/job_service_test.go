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
