package supervisord

import (
	"strings"
	"testing"
)

func TestRedisPersistenceArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		points []RDBSavePoint
		want   []string
	}{
		{
			name:   "ephemeral",
			points: nil,
			want:   []string{`--save ""`, `--appendonly no`},
		},
		{
			name:   "default rdb",
			points: DefaultRDBSavePoints,
			want:   []string{`--appendonly no`, `--save ""`, `--save 60 100`},
		},
		{
			name: "multiple save points",
			points: []RDBSavePoint{
				{Seconds: 60, Changes: 100},
				{Seconds: 300, Changes: 10},
			},
			want: []string{`--appendonly no`, `--save ""`, `--save 60 100`, `--save 300 10`},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := redisPersistenceArgs(tt.points)
			for _, fragment := range tt.want {
				if !strings.Contains(got, fragment) {
					t.Fatalf("redisPersistenceArgs() = %q, missing %q", got, fragment)
				}
			}
		})
	}
}

func TestResolveRDBSavePoints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		noPersist  bool
		flagValues []string
		want       []RDBSavePoint
		wantErr    bool
	}{
		{name: "default", want: DefaultRDBSavePoints},
		{name: "no persist flag", noPersist: true, want: nil},
		{name: "explicit empty", flagValues: []string{""}, want: nil},
		{name: "custom", flagValues: []string{"300:10"}, want: []RDBSavePoint{{Seconds: 300, Changes: 10}}},
		{name: "invalid", flagValues: []string{"bad"}, wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ResolveRDBSavePoints(tt.noPersist, tt.flagValues)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveRDBSavePoints() error = %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("got[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestRedisProgramSpec_defaultRDB(t *testing.T) {
	t.Parallel()

	spec := redisProgramSpec(RedisConfig{
		Port:          6379,
		RDBSavePoints: DefaultRDBSavePoints,
	})
	for _, fragment := range []string{
		"redis-server",
		"--port 6379",
		`--appendonly no`,
		`--save ""`,
		`--save 60 100`,
	} {
		if !strings.Contains(spec.Command, fragment) {
			t.Fatalf("command = %q, missing %q", spec.Command, fragment)
		}
	}
}
