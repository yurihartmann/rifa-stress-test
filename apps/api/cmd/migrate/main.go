package main

import (
	"log/slog"
	"os"

	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/infra/database"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/infra/setting"
)

func main() {
	settings, err := setting.LoadMigrate()
	if err != nil {
		slog.Error("migrate", "error", err.Error())
		os.Exit(1)
	}
	db, err := database.Open(settings.DatabaseURL)
	if err != nil {
		slog.Error("migrate", "error", err.Error())
		os.Exit(1)
	}
	defer func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	}()
	if err := database.Migrate(db); err != nil {
		slog.Error("migrate", "error", err.Error())
		os.Exit(1)
	}
	slog.Info("migration complete")
}
