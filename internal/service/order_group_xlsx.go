package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"xraytool/internal/model"

	"github.com/xuri/excelize/v2"
)

func (s *OrderService) GroupSocks5TemplateXLSX(orderID uint) ([]byte, string, error) {
	head, children, err := s.loadGroupHeadAndChildren(orderID)
	if err != nil {
		return nil, "", err
	}
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	headers := []string{"序号", "IP", "Port", "Username", "Password"}
	for i, title := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, title)
	}
	for idx, child := range children {
		line := idx + 2
		item := child.Items[0]
		_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", line), idx+1)
		_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", line), strings.TrimSpace(item.ForwardAddress))
		_ = f.SetCellValue(sheet, fmt.Sprintf("C%d", line), item.ForwardPort)
		_ = f.SetCellValue(sheet, fmt.Sprintf("D%d", line), strings.TrimSpace(item.ForwardUsername))
		_ = f.SetCellValue(sheet, fmt.Sprintf("E%d", line), strings.TrimSpace(item.ForwardPassword))
	}
	_ = f.SetColWidth(sheet, "A", "A", 8)
	_ = f.SetColWidth(sheet, "B", "E", 22)
	body, err := f.WriteToBuffer()
	if err != nil {
		return nil, "", err
	}
	return body.Bytes(), fmt.Sprintf("group-%d-socks5-template.xlsx", head.ID), nil
}

func (s *OrderService) GroupCredentialsTemplateXLSX(orderID uint) ([]byte, string, error) {
	head, children, err := s.loadGroupHeadAndChildren(orderID)
	if err != nil {
		return nil, "", err
	}
	protocol := exportOrderProtocol(*head)
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	headers := []string{"序号"}
	switch protocol {
	case model.DedicatedFeatureVmess, model.DedicatedFeatureVless:
		headers = append(headers, "UUID")
	case model.DedicatedFeatureShadowsocks:
		headers = append(headers, "Password")
	default:
		headers = append(headers, "Username", "Password", "UUID(可空)")
	}
	for i, title := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, title)
	}
	for idx, child := range children {
		line := idx + 2
		item := child.Items[0]
		_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", line), idx+1)
		switch protocol {
		case model.DedicatedFeatureVmess, model.DedicatedFeatureVless:
			_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", line), strings.TrimSpace(item.VmessUUID))
		case model.DedicatedFeatureShadowsocks:
			_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", line), strings.TrimSpace(item.Password))
		default:
			_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", line), strings.TrimSpace(item.Username))
			_ = f.SetCellValue(sheet, fmt.Sprintf("C%d", line), strings.TrimSpace(item.Password))
			_ = f.SetCellValue(sheet, fmt.Sprintf("D%d", line), strings.TrimSpace(item.VmessUUID))
		}
	}
	_ = f.SetColWidth(sheet, "A", "A", 8)
	if protocol == model.DedicatedFeatureMixed {
		_ = f.SetColWidth(sheet, "B", "D", 28)
	} else {
		_ = f.SetColWidth(sheet, "B", "B", 36)
	}
	body, err := f.WriteToBuffer()
	if err != nil {
		return nil, "", err
	}
	return body.Bytes(), fmt.Sprintf("group-%d-credentials-template.xlsx", head.ID), nil
}

func (s *OrderService) UpdateGroupSocks5FromXLSX(ctx context.Context, orderID uint, body []byte) error {
	lines, err := parseGroupSocks5LinesFromXLSX(body)
	if err != nil {
		return err
	}
	return s.UpdateGroupSocks5(ctx, orderID, strings.Join(lines, "\n"))
}

func (s *OrderService) UpdateGroupCredentialsFromXLSX(ctx context.Context, orderID uint, body []byte) error {
	head, _, err := s.loadGroupHeadAndChildren(orderID)
	if err != nil {
		return err
	}
	protocol := exportOrderProtocol(*head)
	lines, err := parseGroupCredentialLinesFromXLSX(body, protocol)
	if err != nil {
		return err
	}
	return s.UpdateGroupCredentials(ctx, orderID, strings.Join(lines, "\n"), false)
}

func (s *OrderService) loadGroupHeadAndChildren(orderID uint) (*model.Order, []model.Order, error) {
	head := &model.Order{}
	if err := s.db.First(head, orderID).Error; err != nil {
		return nil, nil, err
	}
	if !head.IsGroupHead {
		return nil, nil, errors.New("only group head order supports this action")
	}
	children := []model.Order{}
	if err := s.db.Preload("Items").Where("parent_order_id = ?", head.ID).Order("sequence_no asc, id asc").Find(&children).Error; err != nil {
		return nil, nil, err
	}
	if len(children) == 0 {
		return nil, nil, errors.New("group has no child orders")
	}
	for _, child := range children {
		if len(child.Items) == 0 {
			return nil, nil, fmt.Errorf("child order %d has no item", child.ID)
		}
	}
	return head, children, nil
}

func parseGroupSocks5LinesFromXLSX(body []byte) ([]string, error) {
	book, err := excelize.OpenReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("read xlsx failed: %w", err)
	}
	sheet := book.GetSheetName(0)
	if sheet == "" {
		return nil, errors.New("xlsx has no sheet")
	}
	rows, err := book.GetRows(sheet)
	if err != nil {
		return nil, err
	}
	lines := make([]string, 0)
	for idx := 1; idx < len(rows); idx++ {
		row := rows[idx]
		ip := trimCell(row, 1)
		portRaw := trimCell(row, 2)
		user := trimCell(row, 3)
		pass := trimCell(row, 4)
		if ip == "" && portRaw == "" && user == "" && pass == "" {
			continue
		}
		if ip == "" || portRaw == "" || user == "" || pass == "" {
			return nil, fmt.Errorf("row %d incomplete", idx+1)
		}
		port, err := parseExcelInt(portRaw)
		if err != nil || port <= 0 || port > 65535 {
			return nil, fmt.Errorf("row %d invalid port", idx+1)
		}
		lines = append(lines, fmt.Sprintf("%s:%d:%s:%s", ip, port, user, pass))
	}
	if len(lines) == 0 {
		return nil, errors.New("xlsx has no valid rows")
	}
	return lines, nil
}

func parseGroupCredentialLinesFromXLSX(body []byte, protocol string) ([]string, error) {
	book, err := excelize.OpenReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("read xlsx failed: %w", err)
	}
	sheet := book.GetSheetName(0)
	if sheet == "" {
		return nil, errors.New("xlsx has no sheet")
	}
	rows, err := book.GetRows(sheet)
	if err != nil {
		return nil, err
	}
	lines := make([]string, 0)
	for idx := 1; idx < len(rows); idx++ {
		row := rows[idx]
		switch strings.ToLower(strings.TrimSpace(protocol)) {
		case model.DedicatedFeatureVmess, model.DedicatedFeatureVless:
			uuid := trimCell(row, 1)
			if uuid == "" {
				continue
			}
			lines = append(lines, uuid)
		case model.DedicatedFeatureShadowsocks:
			pass := trimCell(row, 1)
			if pass == "" {
				continue
			}
			lines = append(lines, pass)
		default:
			user := trimCell(row, 1)
			pass := trimCell(row, 2)
			uuid := trimCell(row, 3)
			if user == "" && pass == "" && uuid == "" {
				continue
			}
			if user == "" || pass == "" {
				return nil, fmt.Errorf("row %d incomplete", idx+1)
			}
			if uuid == "" {
				lines = append(lines, fmt.Sprintf("%s:%s", user, pass))
				continue
			}
			lines = append(lines, fmt.Sprintf("%s:%s:%s", user, pass, uuid))
		}
	}
	if len(lines) == 0 {
		return nil, errors.New("xlsx has no valid rows")
	}
	return lines, nil
}

func trimCell(row []string, index int) string {
	if index < 0 || index >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[index])
}

func parseExcelInt(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, errors.New("empty")
	}
	if !strings.Contains(raw, ".") {
		return strconv.Atoi(raw)
	}
	fv, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, err
	}
	return int(fv), nil
}
