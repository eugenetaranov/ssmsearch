package main

import (
	"reflect"
	"testing"
)

func TestReorderArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "flags already first",
			args: []string{"ssmsearch", "-s", "-p", "/payzoff", "dev", "payzoff"},
			want: []string{"ssmsearch", "-s", "-p", "/payzoff", "dev", "payzoff"},
		},
		{
			name: "flags after positional",
			args: []string{"ssmsearch", "-s", "dev", "payzoff", "-p", "/payzoff"},
			want: []string{"ssmsearch", "-s", "-p", "/payzoff", "dev", "payzoff"},
		},
		{
			name: "mixed flags and positional",
			args: []string{"ssmsearch", "-s", "dev", "-r", "payzoff", "-p", "/payzoff"},
			want: []string{"ssmsearch", "-s", "-r", "-p", "/payzoff", "dev", "payzoff"},
		},
		{
			name: "region flag with value",
			args: []string{"ssmsearch", "-s", "term", "-region", "us-west-2"},
			want: []string{"ssmsearch", "-s", "-region", "us-west-2", "term"},
		},
		{
			name: "profile flag with value",
			args: []string{"ssmsearch", "-s", "term", "-profile", "prod"},
			want: []string{"ssmsearch", "-s", "-profile", "prod", "term"},
		},
		{
			name: "boolean flags only",
			args: []string{"ssmsearch", "-l", "-r"},
			want: []string{"ssmsearch", "-l", "-r"},
		},
		{
			name: "no flags",
			args: []string{"ssmsearch", "term1", "term2"},
			want: []string{"ssmsearch", "term1", "term2"},
		},
		{
			name: "program name only",
			args: []string{"ssmsearch"},
			want: []string{"ssmsearch"},
		},
		{
			name: "value flag at end without value",
			args: []string{"ssmsearch", "-s", "term", "-p"},
			want: []string{"ssmsearch", "-s", "-p", "term"},
		},
		{
			name: "multiple value-taking flags",
			args: []string{"ssmsearch", "-s", "term", "-p", "/app", "-region", "us-east-1", "-profile", "dev"},
			want: []string{"ssmsearch", "-s", "-p", "/app", "-region", "us-east-1", "-profile", "dev", "term"},
		},
		{
			name: "flag value starting with dash",
			args: []string{"ssmsearch", "-l", "-p", "-staging"},
			want: []string{"ssmsearch", "-l", "-p", "-staging"},
		},
		{
			name: "positional that looks like flag value",
			args: []string{"ssmsearch", "-s", "us-west-2", "-region", "eu-west-1"},
			want: []string{"ssmsearch", "-s", "-region", "eu-west-1", "us-west-2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reorderArgs(tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("reorderArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}
