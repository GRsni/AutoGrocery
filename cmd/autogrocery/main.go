package main

import (
	"autoGrocery/internal/dia"
	"time"
)

type CredentialsConfig struct {
	Sheets struct {
		MainID string `json:"main-id"`
	} `json:"sheets"`
}

func main() {
	cookiesPath := "config/credentials/cookies-www-dia-es.txt"
	credentialsPath := "config/credentials/credentials.json"
	//ctx := context.Background()
	//
	// tokFile := "config/token.json"
	//srv, err := sheets_handler.GetSheetService(ctx, tokFile, credsFile)
	//if err != nil {
	//	log.Fatalf("Unable to retrieve Sheets client: %v", err)
	//}
	//
	//b, err := os.ReadFile(credsFile)
	//if err != nil {
	//	log.Fatalf("Unable to read client secret file: %v", err)
	//}
	//var config CredentialsConfig
	//if err := json.Unmarshal(b, &config); err != nil {
	//	fmt.Println("Failed to unmarshal credentials config:", err)
	//	return
	//}
	//
	//year, month, day := time.Now().Date()
	//fmt.Println("Current date:", year, constants.FromTimeMonth(month), day)
	//yearStr := fmt.Sprintf("%02d", year%100)
	//readRange := fmt.Sprintf("%s %s!%s", string(constants.FromTimeMonth(month)), yearStr, "A1:F92")
	//resp, err := srv.Spreadsheets.Values.Get(config.Sheets.MainID, readRange).Do()
	//if err != nil {
	//	log.Fatalf("Unable to retrieve data from sheet: %v", err)
	//}
	//
	//if len(resp.Values) == 0 {
	//	fmt.Println("No data found.")
	//} else {
	//	for i, row := range resp.Values {
	//		for j, cell := range row {
	//			fmt.Printf("[%d][%d]: %v\n", i, j, cell)
	//		}
	//		fmt.Println()
	//	}
	//}

	diaPage, cleanup, err := dia.LoginToDia(credentialsPath, cookiesPath)
	if err != nil {
		cleanup()
		return
	}
	dia.GetTicketList(diaPage, time.Now())
	defer cleanup()

	diaPage.Close()

}
