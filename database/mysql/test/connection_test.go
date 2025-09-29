package database_test

import (
	"testing"
	"time"

	"github.com/tuan-dd/go-pkg/database/mysql"
)

func TestNewConnection(t *testing.T) {
	type args struct {
		cfg *mysql.SQLConfig
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Valid configuration",
			args: args{
				cfg: &mysql.SQLConfig{
					Host:            "localhost",
					Port:            3306,
					Username:        "root",
					Password:        "123456",
					DBname:          "ssm-account",
					SSLMode:         "disable",
					MaxConns:        10,
					MaxConnIdleTime: 5,
					MaxIdleConns:    5,
					MaxConnLifetime: uint64(30 * time.Minute),
				},
			},
			wantErr: false,
		},
		// {
		// 	name: "Invalid host",
		// 	args: args{
		// 		cfg: &mysql.SQLConfig{
		// 			Host:            "invalid_host",
		// 			Port:            5432,
		// 			Username:        "mysql",
		// 			Password:        "password",
		// 			DBname:          "testdb",
		// 			SSLMode:         "disable",
		// 			MaxConns:        10,
		// 			MaxConnIdleTime: 5,
		// 			MaxConnLifetime: uint64(30 * time.Minute),
		// 		},
		// 	},
		// 	wantErr: true,
		// },
		// {
		// 	name: "Empty configuration",
		// 	args: args{
		// 		cfg: &mysql.SQLConfig{},
		// 	},
		// 	wantErr: true,
		// },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mysql.NewConnection(tt.args.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewConnection() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && got == nil {
				t.Errorf("NewConnection() got nil connection for valid input")
			}
		})
	}
}
