// +build integration

/*
Real-time Online/Offline Charging System (OCS) for Telecom & ISP environments
Copyright (C) ITsysCOM GmbH

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>
*/

package ees

import (
	"fmt"
	"net/rpc"
	"path"
	"testing"
	"time"

	"github.com/cgrates/cgrates/utils"

	"github.com/jinzhu/gorm"

	"github.com/cgrates/cgrates/config"
	"github.com/cgrates/cgrates/engine"
)

var (
	sqlEeConfigDir string
	sqlEeCfgPath   string
	sqlEeCfg       *config.CGRConfig
	sqlEeRpc       *rpc.Client
	db2            *gorm.DB
	dbConnString   = "cgrates:CGRateS.org@tcp(127.0.0.1:3306)/%s?charset=utf8&loc=Local&parseTime=true&sql_mode='ALLOW_INVALID_DATES'"

	sTestsSqlEe = []func(t *testing.T){
		testCreateDirectory,
		testSqlEeCreateTable,
		testSqlEeLoadConfig,
		testSqlEeResetDataDB,
		testSqlEeResetStorDb,
		testSqlEeStartEngine,
		testSqlEeRPCConn,
		testSqlEeExportEvent,
		testSqlEeVerifyExportedEvent,
		testStopCgrEngine,
		testCleanDirectory,
	}
)

func TestSqlEeExport(t *testing.T) {
	sqlEeConfigDir = "ees"
	for _, stest := range sTestsSqlEe {
		t.Run(sqlEeConfigDir, stest)
	}
}

// create a struct serve as model for *sql exporter
type testModelSql struct {
	ID         int64
	Cgrid      string
	AnswerTime time.Time
	Usage      int64
	Cost       float64
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  *time.Time
}

func (_ *testModelSql) TableName() string {
	return "expTable"
}

type nopLogger struct{}

func (nopLogger) Print(values ...interface{}) {}

func testSqlEeCreateTable(t *testing.T) {
	var err error

	if db2, err = gorm.Open("mysql", fmt.Sprintf(dbConnString, "exportedDatabase")); err != nil {
		t.Fatal(err)
	}
	db2.SetLogger(new(nopLogger))

	if _, err = db2.DB().Exec(`CREATE DATABASE IF NOT EXISTS exportedDatabase;`); err != nil {
		t.Fatal(err)
	}
	tx := db2.Begin()
	if !tx.HasTable("expTable") {
		tx = tx.CreateTable(new(testModelSql))
		if err = tx.Error; err != nil {
			tx.Rollback()
			t.Fatal(err)
		}
	}
	tx.Commit()
}

func testSqlEeLoadConfig(t *testing.T) {
	var err error
	sqlEeCfgPath = path.Join(*dataDir, "conf", "samples", sqlEeConfigDir)
	if sqlEeCfg, err = config.NewCGRConfigFromPath(sqlEeCfgPath); err != nil {
		t.Error(err)
	}
}

func testSqlEeResetDataDB(t *testing.T) {
	if err := engine.InitDataDb(sqlEeCfg); err != nil {
		t.Fatal(err)
	}
}

func testSqlEeResetStorDb(t *testing.T) {
	if err := engine.InitStorDb(sqlEeCfg); err != nil {
		t.Fatal(err)
	}
}

func testSqlEeStartEngine(t *testing.T) {
	if _, err := engine.StopStartEngine(sqlEeCfgPath, *waitRater); err != nil {
		t.Fatal(err)
	}
}

func testSqlEeRPCConn(t *testing.T) {
	var err error
	sqlEeRpc, err = newRPCClient(sqlEeCfg.ListenCfg())
	if err != nil {
		t.Fatal(err)
	}
}

func testSqlEeExportEvent(t *testing.T) {
	eventVoice := &utils.CGREventWithEeIDs{
		EeIDs: []string{"SQLExporter"},
		CGREventWithOpts: &utils.CGREventWithOpts{
			CGREvent: &utils.CGREvent{
				Tenant: "cgrates.org",
				ID:     "voiceEvent",
				Time:   utils.TimePointer(time.Now()),
				Event: map[string]interface{}{
					utils.CGRID:       utils.Sha1("dsafdsaf", time.Unix(1383813745, 0).UTC().String()),
					utils.ToR:         utils.VOICE,
					utils.OriginID:    "dsafdsaf",
					utils.OriginHost:  "192.168.1.1",
					utils.RequestType: utils.META_RATED,
					utils.Tenant:      "cgrates.org",
					utils.Category:    "call",
					utils.Account:     "1001",
					utils.Subject:     "1001",
					utils.Destination: "1002",
					utils.SetupTime:   time.Unix(1383813745, 0).UTC(),
					utils.AnswerTime:  time.Unix(1383813746, 0).UTC(),
					utils.Usage:       10 * time.Second,
					utils.RunID:       utils.MetaDefault,
					utils.Cost:        1.01,
					"ExtraFields": map[string]string{"extra1": "val_extra1",
						"extra2": "val_extra2", "extra3": "val_extra3"},
				},
			},
		},
	}

	var reply map[string]utils.MapStorage
	if err := sqlEeRpc.Call(utils.EeSv1ProcessEvent, eventVoice, &reply); err != nil {
		t.Error(err)
	}
	time.Sleep(10 * time.Millisecond)
}

func testSqlEeVerifyExportedEvent(t *testing.T) {
	var result int64
	db2.Table("expTable").Count(&result)
	if result != 1 {
		t.Fatal("Expected table to have only one result ", result)
	}
}