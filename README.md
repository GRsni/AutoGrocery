# autoGrocery

A Go-based application for automatically reading grocery data from Mercadona, Carrefour, and Dia, and displaying grocery
data from Google Sheets.

## 📋 Overview

`autoGrocery` is a command-line tool that connects to a Google Spreadsheet via the Sheets API, authenticates using
OAuth2, and retrieves grocery data organized by month and year.

## ✨ Features

- **Google Sheets Integration**: Read data directly from Google Sheets using the official Sheets API
- **OAuth2 Authentication**: Secure authentication with Google credentials
- **Monthly Organization**: Data is organized by months and years for easy retrieval
- **Simple CLI Interface**: Easy-to-use command-line application
- **Error Handling**: Comprehensive error handling for reliable execution

## 🛠️ Prerequisites

- Go 1.25 or higher
- A Google Cloud project with Sheets API enabled
- OAuth2 credentials (client secret JSON file)
- A Google Spreadsheet ID with read-only access

## 🔧 Setup & Configuration

### 1. Google Cloud Setup

1. Create a Google Cloud project at [Google Cloud Console](https://console.cloud.google.com/)
2. Enable the **Google Sheets API**
3. Create OAuth2 credentials (Client Secret)
4. Download and save the credentials as `config/credentials.json`

### 2. Application Token

After first run, a token file will be created at `config/token.json`. Delete this file if you need to regenerate
authentication tokens.

### 3. Spreadsheet Configuration

Your Google Spreadsheet should have:

- **Spreadsheet ID**: `1QG3-vZCIqXut4oz_4r4OkGJeMV91SRZ5od_3rLYj1Ao`
- **Data Range**: Columns A-F, rows 1-92
- **Month Headers**: First row contains month names

## 📁 Project Structure

```
autoGrocery/ 
├── cmd/autogrocery/ # Main application entry point 
│ └── main.go 
├── config/ # Configuration files 
│ 
├── credentials.json # OAuth2 client credentials 
│ └── token.json # OAuth2 token (generated on first run) 
├── internal/ 
│ 
├── sheets/ # Google Sheets API integration 
│ └── token/ # Token management 
│ └── manager.go # OAuth2 token handling 
├── pkg/ 
│ └── constants/ # Application constants 
│ └── months.go # Month name mappings 
├── go.mod # Go module definition 
└── .gitignore # Git ignore rules
```

## 🚀 Usage

Build the application:

```bash
go build -o autogrocery ./cmd/autogrocery
```

Run the application:

``` bash
./autogrocery
```

The application will:

- Read your Google Sheets data for the current month
- Display the data in a formatted output
- Show all grocery items with their details

## 🔍 Technical Details

### Dependencies

- `golang.org/x/oauth2/google` - OAuth2 authentication
- `google.golang.org/api/sheets/v4` - Google Sheets API client

### Authentication Flow

1. Application loads OAuth2 credentials from `config/credentials.json`
2. Creates a Google Sheets client service
3. Authenticates and obtains an access token
4. Saves the token to for subsequent runs `config/token.json`
5. Queries the spreadsheet with the configured range

## ⚠️ Security Notes

- contains sensitive OAuth2 credentials - keep it secure `credentials.json`
- stores authentication tokens - restrict file permissions `token.json`
- The application uses read-only scope: `https://www.googleapis.com/auth/spreadsheets.readonly`

## 🧪 Testing

Run tests (if available):

```
go test ./...
```

## 🤝 Contributing

- Fork the repository
- Create a feature branch
- Commit your changes
- Push to the branch
- Open a Pull Request

## 📞 Support

For issues or questions, please open an issue on the repository.