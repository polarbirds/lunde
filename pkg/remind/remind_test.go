package remind

import (
	"testing"
	"time"
)

func Test_parseStringToDuration(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name       string
		args       args
		wantDuratn time.Duration
		wantErr    bool
	}{
		{
			name:       "test 14h and getting 14 hours",
			args:       args{"14h"},
			wantDuratn: 14 * time.Hour,
			wantErr:    false,
		}, {
			name:       "test 12months and getting 12 months",
			args:       args{"12months"},
			wantDuratn: 12 * time.Minute * 43800,
			wantErr:    false,
		}, {
			name:       "test 12 and getting 12 minutes",
			args:       args{"12"},
			wantDuratn: 12 * time.Minute,
			wantErr:    false,
		}, {
			name:       "test y2 and getting 2 years",
			args:       args{"y2"},
			wantDuratn: 2 * time.Minute * 525600,
			wantErr:    false,
		}, {
			name:       "test 1h3 and getting 13 hours",
			args:       args{"1h3"},
			wantDuratn: 13 * time.Hour,
			wantErr:    false,
		}, {
			name:    "test 0h and getting error",
			args:    args{"0h"},
			wantErr: true,
		}, {
			name:    "test blank and getting error",
			args:    args{""},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDuratn, err := parseStringToDuration(tt.args.s)
			if (err != nil) != tt.wantErr {
				t.Errorf("splitStringIntoNumberAndDenot() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotDuratn != tt.wantDuratn {
				t.Errorf("splitStringIntoNumberAndDenot() gotDuratn = %v, want %v",
					gotDuratn, tt.wantDuratn)
			}
		})
	}
}
