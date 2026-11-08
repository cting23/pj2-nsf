package nsfContext

import (
	"encoding/json"
	"fmt"

	"nsf/internal/db"
	"nsf/logger"
)

// 每筆服務紀錄包含 NF 名稱、IP 和 Port
type serviceRecord struct {
	NfName     string `json:"nf_name"`
	SbiAddress string `json:"sbi_address"`
	SbiPort    int    `json:"sbi_port"`
}

type serviceContextIE struct {
	db.DbIf
	*logger.NsfLogger
}

type serviceContext struct {
	db.DbIf
	*logger.NsfLogger
}

// 初始化 Service Context
func newServiceContext(ie *serviceContextIE) *serviceContext {
	return &serviceContext{
		DbIf:      ie.DbIf,
		NsfLogger: ie.NsfLogger,
	}
}

// 釋放 Service Context
// DB 由 dbContext 統一負責關閉
func (s *serviceContext) release() {
	s.NsfLogger.SvcLog.Infoln("Release serviceContext...")
	s.NsfLogger.SvcLog.Infoln("serviceContext released")
}

// Register：把服務資料寫入資料庫
func (s *serviceContext) registerServices(
	nfName, sbiAddr string,
	sbiPort int,
	services []string,
) error {
	s.NsfLogger.SvcLog.Tracef("Registering %s services", nfName)

	record := serviceRecord{
		NfName:     nfName,
		SbiAddress: sbiAddr,
		SbiPort:    sbiPort,
	}

	// 將 NF 資訊轉為 JSON
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal service record: %w", err)
	}

	// 一次儲存所有服務
	entries := make(map[string]string)
	for _, service := range services {
		entries[service] = string(data)
	}

	if len(entries) == 0 {
		return nil
	}

	if err := s.DbIf.SaveAll(entries); err != nil {
		return fmt.Errorf("register services for %s: %w", nfName, err)
	}

	return nil
}

// Deregister：刪除指定 NF 註冊的服務
func (s *serviceContext) deregisterServices(
	nfName string,
	services []string,
) error {
	s.NsfLogger.SvcLog.Tracef("De-registering %s services", nfName)

	for _, service := range services {
		// 先確認服務存在
		exists, err := s.DbIf.Exist(service)
		if err != nil {
			return fmt.Errorf("check service %s: %w", service, err)
		}
		if !exists {
			continue
		}

		// 確認這筆服務屬於指定的 NF
		value, err := s.DbIf.Load(service)
		if err != nil {
			return fmt.Errorf("load service %s: %w", service, err)
		}

		var record serviceRecord
		if err := json.Unmarshal([]byte(value), &record); err != nil {
			return fmt.Errorf("decode service %s: %w", service, err)
		}

		if record.NfName != nfName {
			continue
		}

		if err := s.DbIf.Remove(service); err != nil {
			return fmt.Errorf("remove service %s: %w", service, err)
		}
	}

	return nil
}

// Discovery：依服務名稱查詢 NF 資訊
func (s *serviceContext) discoverService(
	service string,
) (string, string, int, error) {
	s.NsfLogger.SvcLog.Tracef("Discovering service %s", service)

	exists, err := s.DbIf.Exist(service)
	if err != nil {
		return "", "", 0, fmt.Errorf("check service %s: %w", service, err)
	}
	if !exists {
		return "", "", 0, fmt.Errorf("service %s not found", service)
	}

	value, err := s.DbIf.Load(service)
	if err != nil {
		return "", "", 0, fmt.Errorf("load service %s: %w", service, err)
	}

	var record serviceRecord
	if err := json.Unmarshal([]byte(value), &record); err != nil {
		return "", "", 0, fmt.Errorf("decode service %s: %w", service, err)
	}

	return record.NfName, record.SbiAddress, record.SbiPort, nil
}
