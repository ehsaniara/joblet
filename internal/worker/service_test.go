package worker

import (
	"context"
	pb "job-worker/api/gen"
	"reflect"
	"testing"
)

func TestStartJob(t *testing.T) {
	type args struct {
		ctx context.Context
		job *pb.Job
	}
	tests := []struct {
		name    string
		args    args
		want    *pb.Job
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &service{}
			got, err := s.StartJob(tt.args.ctx, tt.args.job)
			if (err != nil) != tt.wantErr {
				t.Errorf("StartJob() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("StartJob() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStopJob(t *testing.T) {
	type args struct {
		jobId string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &service{}
			if err := s.StopJob(tt.args.jobId); (err != nil) != tt.wantErr {
				t.Errorf("StopJob() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
