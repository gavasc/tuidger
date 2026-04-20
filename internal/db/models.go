package db

type Transaction struct {
	ID               *int64
	Type             string // "expense" | "revenue"
	Desc             string
	Cat              string
	Val              float64
	Date             string // YYYY-MM-DD
	AccountID        *int64
	InstallmentID    *int64
	InstallmentIndex *int64
}

type Installment struct {
	ID            *int64
	Desc          string
	Cat           string
	TotalVal      float64
	NInstallments int64
	StartDate     string
	AccountID     *int64
	PaidCount     *int64   // computed
	MonthlyVal    *float64 // computed = TotalVal/NInstallments
}

type Account struct {
	ID             *int64
	Name           string
	InitialBalance float64
}

type AccountBalance struct {
	ID      int64
	Name    string
	Balance float64
}

type Transfer struct {
	ID              *int64
	FromAccountID   int64
	ToAccountID     int64
	FromAccountName *string
	ToAccountName   *string
	Amount          float64
	Date            string
	Desc            string
}

type BackupConfig struct {
	Provider  string `yaml:"provider"` // github/gitlab/forgejo/gitea/custom
	Host      string `yaml:"host"`
	Repo      string `yaml:"repo"`
	Token     string `yaml:"token"`
	RemoteURL string `yaml:"remote_url"` // if set, used directly (e.g. SSH remotes)
}
