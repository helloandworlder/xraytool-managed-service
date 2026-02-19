package service

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"xraytool/internal/config"

	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type BackupInfo struct {
	Name      string    `json:"name"`
	SizeBytes int64     `json:"size_bytes"`
	UpdatedAt time.Time `json:"updated_at"`
}

type BackupService struct {
	cfg config.Config
	db  *gorm.DB
	log *zap.Logger
}

func NewBackupService(cfg config.Config, db *gorm.DB, log *zap.Logger) *BackupService {
	return &BackupService{cfg: cfg, db: db, log: log}
}

func (s *BackupService) ListBackups() ([]BackupInfo, error) {
	entries, err := os.ReadDir(s.cfg.BackupDir)
	if err != nil {
		return nil, err
	}
	out := make([]BackupInfo, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".db") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, BackupInfo{
			Name:      e.Name(),
			SizeBytes: info.Size(),
			UpdatedAt: info.ModTime(),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}

func (s *BackupService) CreateBackup() (BackupInfo, error) {
	name := fmt.Sprintf("backup-%s.db", time.Now().Format("20060102-150405"))
	target := filepath.Join(s.cfg.BackupDir, name)
	if err := s.createBackupTo(target); err != nil {
		return BackupInfo{}, err
	}
	st, err := os.Stat(target)
	if err != nil {
		return BackupInfo{}, err
	}
	return BackupInfo{Name: name, SizeBytes: st.Size(), UpdatedAt: st.ModTime()}, nil
}

func (s *BackupService) CreateTempExport() (filePath string, downloadName string, err error) {
	name := fmt.Sprintf("xraytool-backup-%s.db", time.Now().Format("20060102-150405"))
	tmpPath := filepath.Join(os.TempDir(), name)
	if err := s.createBackupTo(tmpPath); err != nil {
		return "", "", err
	}
	return tmpPath, name, nil
}

func (s *BackupService) createBackupTo(target string) error {
	if err := os.MkdirAll(s.cfg.BackupDir, 0o755); err != nil {
		return err
	}
	_ = os.Remove(target)

	quoted := strings.ReplaceAll(target, "'", "''")
	if err := s.db.Exec("VACUUM INTO '" + quoted + "'").Error; err != nil {
		return err
	}
	return nil
}

func (s *BackupService) BackupPath(name string) (string, error) {
	clean := filepath.Base(strings.TrimSpace(name))
	if clean == "" || clean == "." || clean == ".." {
		return "", fmt.Errorf("invalid backup name")
	}
	if !strings.HasSuffix(clean, ".db") {
		return "", fmt.Errorf("backup file must end with .db")
	}
	path := filepath.Join(s.cfg.BackupDir, clean)
	if _, err := os.Stat(path); err != nil {
		return "", err
	}
	return path, nil
}

func (s *BackupService) DeleteBackup(name string) error {
	path, err := s.BackupPath(name)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

func (s *BackupService) RestoreBackup(name string) error {
	path, err := s.BackupPath(name)
	if err != nil {
		return err
	}
	ok, err := checkSQLiteIntegrity(path)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("backup integrity check failed")
	}

	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	if err := sqlDB.Close(); err != nil {
		return err
	}

	tmp := s.cfg.DBPath + ".restore.tmp"
	if err := copyFile(path, tmp); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.cfg.DBPath); err != nil {
		return err
	}

	s.log.Info("database restored from backup", zap.String("backup", name))
	return nil
}

func checkSQLiteIntegrity(path string) (bool, error) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return false, err
	}
	type row struct{ Value string }
	rows := []row{}
	if err := db.Raw("PRAGMA integrity_check;").Scan(&rows).Error; err != nil {
		return false, err
	}
	if len(rows) == 0 {
		return false, nil
	}
	for _, r := range rows {
		if strings.ToLower(strings.TrimSpace(r.Value)) != "ok" {
			return false, nil
		}
	}
	return true, nil
}

func copyFile(src, dst string) error {
	from, err := os.Open(src)
	if err != nil {
		return err
	}
	defer from.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	to, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer to.Close()

	if _, err := io.Copy(to, from); err != nil {
		return err
	}
	return to.Sync()
}
