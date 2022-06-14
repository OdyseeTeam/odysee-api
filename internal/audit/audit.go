package audit

import (
	"github.com/OdyseeTeam/odysee-api/internal/monitor"
	"github.com/OdyseeTeam/odysee-api/models"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
)

var logger = monitor.NewModuleLogger("audit")

func LogQuery(userID int, remoteIP string, method string, body []byte) *models.QueryLog {
	qLog := models.QueryLog{Method: method, UserID: null.IntFrom(userID), RemoteIP: remoteIP, Body: null.JSONFrom(body)}
	err := qLog.InsertG(boil.Infer())
	if err != nil {
		logger.Log().Error("cannot insert query log:", err)
	}
	return &qLog
}
