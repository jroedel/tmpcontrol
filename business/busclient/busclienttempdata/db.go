package busclienttempdata

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func (th *TempHandler) create(temp Temperature) error {
	const query = `
		INSERT INTO tmplog
		(
			ExecutionIdentifier,
			ControllerName,
			Timestamp,
			TemperatureInF,
			DesiredTemperatureInF,
			IsHeatingNotCooling,
			TurningOnNotOff,
			HostsPipeSeparated,
			HasBeenSentToServer
		)
		VALUES
			(?, ?, ?, ?, ?, ?, ?, ?, ?)`

	err := th.db.Create(query,
		th.executionID,
		temp.ControllerName,
		temp.Timestamp.Unix(),
		temp.TemperatureInF,
		temp.DesiredTemperatureInF,
		temp.IsHeatingNotCooling,
		temp.TurningOnNotOff,
		temp.HostsPipeSeparated,
		false)
	if err != nil {
		return err
	}

	return nil
}

func (th *TempHandler) queryHasNotBeenSentToServer() ([]Temperature, error) {
	const query = `
		SELECT
			Id,
			ExecutionIdentifier,
			ControllerName,
			Timestamp,
			TemperatureInF,
			DesiredTemperatureInF,
			IsHeatingNotCooling,
			TurningOnNotOff,
			HostsPipeSeparated
		FROM
			tmplog
		WHERE
			HasBeenSentToServer = 0`

	var tempTimestampStr int64
	var tempTmpLog Temperature

	results, err := th.db.Query(query,
		[]any{}, //no params
		&tempTmpLog.DbAutoId,
		&tempTmpLog.ExecutionIdentifier,
		&tempTmpLog.ControllerName,
		&tempTimestampStr,
		&tempTmpLog.TemperatureInF,
		&tempTmpLog.DesiredTemperatureInF,
		&tempTmpLog.IsHeatingNotCooling,
		&tempTmpLog.TurningOnNotOff,
		&tempTmpLog.HostsPipeSeparated)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}

	temps := make([]Temperature, len(results))
	for i, result := range results {
		temps[i].DbAutoId = result[i].(int)
		temps[i].ExecutionIdentifier = result[i].(string)
		temps[i].ControllerName = result[i].(string)
		temps[i].Timestamp = time.Unix(result[i].(int64), 0)
		temps[i].TemperatureInF = result[i].(float32)
		temps[i].DesiredTemperatureInF = result[i].(float32)
		temps[i].IsHeatingNotCooling = result[i].(bool)
		temps[i].TurningOnNotOff = result[i].(bool)
		temps[i].HostsPipeSeparated = result[i].(string)
	}

	return temps, nil
}

func (th *TempHandler) getAverageRecentTemperature(controllerName string, d time.Duration) (float32, error) {
	timestampRef := time.Now().Add(-d).Unix()
	query := `
		SELECT 
		    AVG(TemperatureInF)
		FROM tmplog 
		WHERE 
		    ExecutionIdentifier = ?
		  AND ControllerName = ?
		  AND Timestamp >= ?`
	params := []any{
		th.executionID,
		controllerName,
		timestampRef,
	}
	var avgTemp float32
	err := th.db.QueryRow(query, params, &avgTemp)
	if err != nil {
		return 0, err
	}
	return avgTemp, nil
}

// markTmpLogsAsSentToServer TODO implement a limit to how many can be marked complete at at time
func (th *TempHandler) markTmpLogsAsSentToServer(ids []int) error {
	idListBuilder := strings.Builder{}

	writeLeadingComma := false
	for _, id := range ids {
		if writeLeadingComma {
			idListBuilder.WriteRune(',')
		}
		idListBuilder.WriteString(strconv.Itoa(id))
		writeLeadingComma = true
	}

	idList := idListBuilder.String()
	fmt.Printf("Here's the list of Ids we'll mark as sent: %#v\n", idList)
	query := fmt.Sprintf(`
		UPDATE tmplog
		SET HasBeenSentToServer = TRUE
		WHERE Id IN (%s)`,
		idList)
	err := th.db.ExecuteQuery(query, []any{})
	return err
}
