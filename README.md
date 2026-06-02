# autoGrocery

[![Go Coverage](https://github.com/GRsni/AutoGrocery/wiki/coverage.svg)](https://raw.githack.com/wiki/GRsni/AutoGrocery/coverage.html)

A Go-based application for automatically reading grocery data from Mercadona, Carrefour, and Dia,
and displaying grocery data from Google Sheets.

## 📋 Overview

`autoGrocery` is a command-line tool that:

- Scrapes grocery data from popular Spanish supermarket websites (Mercadona, Carrefour, Dia)
- Stores the data in a Google Spreadsheet via the Sheets API
- Authenticates using OAuth2 for secure access

## ✨ Features

- **Multi-Supermarket Support**: Read data from Mercadona, Carrefour, and Dia
- **Google Sheets Integration**: Store and retrieve data directly from Google Sheets using the
  official Sheets API
- **OAuth2 Authentication**: Secure authentication with Google credentials
- **Monthly Organization**: Data is organized by months and years for easy retrieval
- **Simple CLI Interface**: Easy-to-use command-line application
- **Comprehensive Testing**: Includes unit tests for core functionality
- **Error Handling**: Comprehensive error handling for reliable execution

## 🛠️ Prerequisites

- Go 1.26 or higher
- A Google Cloud project with Sheets API enabled
- OAuth2 credentials (client secret JSON file)
- A Google Spreadsheet ID with read-only access
- Browser automation capabilities (for scraping supermarket sites)

## 📁 Project Structure

```
autoGrocery/ 
├── cmd/autogrocery/ # Main application entry point 
│ └── main.go ├── config/ # Configuration files 
│ └── credentials/ # OAuth2 and cookie credentials 
│ ├── cookies-www-dia-es.txt 
│ ├── credentials.json # OAuth2 client credentials 
│ ├── dia_cookies.txt # Dia-specific cookies 
│ └── token.json # OAuth2 token (generated on first run) 
├── internal/ # Internal packages 
│ ├── carrefour/ # Carrefour integration 
│ │ └── carrefour.go 
│ ├── dia/ # Dia integration with tests 
│ │ ├── dia.go 
│ │ └── dia_test.go 
│ ├── sheets_handler/ # Google Sheets API integration 
│ │ ├── sheethandler.go 
│ │ └── sheethandler_test.go 
│ └── token/ # Token management 
│ ├── manager.go 
│ └── manager_test.go 
├── pkg/ # Public packages 
│ └── constants/ # Application constants 
│ ├── markets.go # Market/supermarket definitions 
│ └── months.go # Month name mappings 
├── images/debug/ # Debug screenshots 
│ ├── carrefour/ 
│ │ ├── details.png 
│ │ ├── login.png 
│ │ └── menu.png 
│ └── dia/ 
│ ├── email-sent.png 
│ ├── login.png 
│ └── username-added.png 
├── .github/workflows/ # CI/CD workflows 
├── go.mod # Go module definition 
├── go.sum # Dependency checksums 
└── README.md # This file
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

1. Authenticate with Google Sheets API
2. Scrape grocery data from configured supermarkets
3. Store the data in your Google Spreadsheet
4. Display a summary of the collected data

## 🔍 Technical Details

### Dependencies

- **Web Scraping & Automation**
    - `github.com/go-rod/rod v0.116.2` - Browser automation
    - `github.com/go-rod/stealth v0.4.9` - Stealth mode for automation

- **Google Services**
    - `golang.org/x/oauth2 v0.36.0` - OAuth2 authentication
    - `google.golang.org/api v0.280.0` - Google Sheets API client

### Authentication Flow

1. Application loads OAuth2 credentials from `config/credentials.json`
2. Creates a Google Sheets client service
3. Authenticates and obtains an access token
4. Saves the token to `config/credentials/token.json` for subsequent runs
5. Queries the spreadsheet with the configured range

### Cookie Management

For supermarket scraping, cookies are managed in:

- `config/credentials/dia_cookies.txt` - Dia-specific session cookies
- Additional cookie files may be required for other supermarkets

## ⚠️ Security Notes

- **Sensitive Credentials**: Contains OAuth2 credentials in `config/credentials/secrets.json` -
  keep it secure and never commit to version control
- **Token Storage**: Stores authentication tokens in `config/credentials/token.json` - restrict file
  permissions (chmod 600)
- **Cookie Files**: Browser cookies contain session data – handle with care
- **Read-Only Scope**: The application uses read-only scope:
  `https://www.googleapis.com/auth/spreadsheets.readonly`

## 🧪 Testing

Run tests to verify functionality:

```bash
 go test ./...
```

Tests are available for:

- Token management (`internal/token/`)
- Sheets handler (`internal/sheets_handler/`)
- Dia integration (`internal/dia/`)

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Commit your changes
4. Push to the branch
5. Open a Pull Request

## 📞 Support

For issues or questions, please open an issue on the repository.