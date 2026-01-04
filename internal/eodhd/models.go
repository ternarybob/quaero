package eodhd

import (
	"time"
)

// EODData represents a single day's end-of-day price data.
type EODData struct {
	Date          time.Time `json:"-"`
	DateStr       string    `json:"date"`
	Open          float64   `json:"open"`
	High          float64   `json:"high"`
	Low           float64   `json:"low"`
	Close         float64   `json:"close"`
	AdjustedClose float64   `json:"adjusted_close"`
	Volume        int64     `json:"volume"`
}

// EODResponse is a slice of EODData.
type EODResponse []EODData

// DividendData represents dividend information.
type DividendData struct {
	Date            time.Time `json:"-"`
	DateStr         string    `json:"date"`
	DeclarationDate string    `json:"declarationDate"`
	RecordDate      string    `json:"recordDate"`
	PaymentDate     string    `json:"paymentDate"`
	Value           float64   `json:"value"`
	UnadjustedValue float64   `json:"unadjustedValue"`
	Currency        string    `json:"currency"`
}

// DividendsResponse is a slice of DividendData.
type DividendsResponse []DividendData

// SplitData represents stock split information.
type SplitData struct {
	Date    time.Time `json:"-"`
	DateStr string    `json:"date"`
	Split   string    `json:"split"` // e.g., "2/1" for 2-for-1 split
}

// SplitsResponse is a slice of SplitData.
type SplitsResponse []SplitData

// NewsItem represents a single news article.
type NewsItem struct {
	Date      time.Time      `json:"-"`
	DateStr   string         `json:"date"`
	Title     string         `json:"title"`
	Content   string         `json:"content"`
	Link      string         `json:"link"`
	Symbols   []string       `json:"symbols"`
	Tags      []string       `json:"tags"`
	Sentiment *NewsSentiment `json:"sentiment,omitempty"`
}

// NewsSentiment represents sentiment analysis data for news.
type NewsSentiment struct {
	Polarity float64 `json:"polarity"`
	Neg      float64 `json:"neg"`
	Neu      float64 `json:"neu"`
	Pos      float64 `json:"pos"`
}

// NewsResponse is a slice of NewsItem.
type NewsResponse []NewsItem

// Exchange represents an exchange from the exchanges list.
type Exchange struct {
	Name         string `json:"Name"`
	Code         string `json:"Code"`
	OperatingMIC string `json:"OperatingMIC"`
	Country      string `json:"Country"`
	Currency     string `json:"Currency"`
	CountryISO2  string `json:"CountryISO2"`
	CountryISO3  string `json:"CountryISO3"`
}

// ExchangesResponse is a slice of Exchange.
type ExchangesResponse []Exchange

// FundamentalsResponse represents the full fundamentals data for a symbol.
type FundamentalsResponse struct {
	General           *GeneralInfo       `json:"General"`
	Highlights        *Highlights        `json:"Highlights"`
	Valuation         *Valuation         `json:"Valuation"`
	Technicals        *Technicals        `json:"Technicals"`
	SplitsDividends   *SplitsDividends   `json:"SplitsDividends"`
	AnalystRatings    *AnalystRatings    `json:"AnalystRatings"`
	Holders           *Holders           `json:"Holders"`
	ESGScores         *ESGScores         `json:"ESGScores"`
	OutstandingShares *OutstandingShares `json:"outstandingShares"`
	Earnings          *Earnings          `json:"Earnings"`
	Financials        *Financials        `json:"Financials"`
}

// GeneralInfo contains general company information.
type GeneralInfo struct {
	Code                  string `json:"Code"`
	Type                  string `json:"Type"`
	Name                  string `json:"Name"`
	Exchange              string `json:"Exchange"`
	CurrencyCode          string `json:"CurrencyCode"`
	CurrencyName          string `json:"CurrencyName"`
	CurrencySymbol        string `json:"CurrencySymbol"`
	CountryName           string `json:"CountryName"`
	CountryISO            string `json:"CountryISO"`
	ISIN                  string `json:"ISIN"`
	CUSIP                 string `json:"CUSIP"`
	CIK                   string `json:"CIK"`
	EmployerIDNumber      string `json:"EmployerIdNumber"`
	FiscalYearEnd         string `json:"FiscalYearEnd"`
	IPODate               string `json:"IPODate"`
	InternationalDomestic string `json:"InternationalDomestic"`
	Sector                string `json:"Sector"`
	Industry              string `json:"Industry"`
	GicSector             string `json:"GicSector"`
	GicGroup              string `json:"GicGroup"`
	GicIndustry           string `json:"GicIndustry"`
	GicSubIndustry        string `json:"GicSubIndustry"`
	HomeCategory          string `json:"HomeCategory"`
	IsDelisted            bool   `json:"IsDelisted"`
	Description           string `json:"Description"`
	Address               string `json:"Address"`
	Phone                 string `json:"Phone"`
	WebURL                string `json:"WebURL"`
	LogoURL               string `json:"LogoURL"`
	FullTimeEmployees     int    `json:"FullTimeEmployees"`
	UpdatedAt             string `json:"UpdatedAt"`
}

// Highlights contains key financial highlights.
type Highlights struct {
	MarketCapitalization       float64 `json:"MarketCapitalization"`
	MarketCapitalizationMln    float64 `json:"MarketCapitalizationMln"`
	EBITDA                     float64 `json:"EBITDA"`
	PERatio                    float64 `json:"PERatio"`
	PEGRatio                   float64 `json:"PEGRatio"`
	WallStreetTargetPrice      float64 `json:"WallStreetTargetPrice"`
	BookValue                  float64 `json:"BookValue"`
	DividendShare              float64 `json:"DividendShare"`
	DividendYield              float64 `json:"DividendYield"`
	EarningsShare              float64 `json:"EarningsShare"`
	EPSEstimateCurrentYear     float64 `json:"EPSEstimateCurrentYear"`
	EPSEstimateNextYear        float64 `json:"EPSEstimateNextYear"`
	EPSEstimateNextQuarter     float64 `json:"EPSEstimateNextQuarter"`
	EPSEstimateCurrentQuarter  float64 `json:"EPSEstimateCurrentQuarter"`
	MostRecentQuarter          string  `json:"MostRecentQuarter"`
	ProfitMargin               float64 `json:"ProfitMargin"`
	OperatingMarginTTM         float64 `json:"OperatingMarginTTM"`
	ReturnOnAssetsTTM          float64 `json:"ReturnOnAssetsTTM"`
	ReturnOnEquityTTM          float64 `json:"ReturnOnEquityTTM"`
	RevenueTTM                 float64 `json:"RevenueTTM"`
	RevenuePerShareTTM         float64 `json:"RevenuePerShareTTM"`
	QuarterlyRevenueGrowthYOY  float64 `json:"QuarterlyRevenueGrowthYOY"`
	GrossProfitTTM             float64 `json:"GrossProfitTTM"`
	DilutedEpsTTM              float64 `json:"DilutedEpsTTM"`
	QuarterlyEarningsGrowthYOY float64 `json:"QuarterlyEarningsGrowthYOY"`
}

// Valuation contains valuation metrics.
type Valuation struct {
	TrailingPE             float64 `json:"TrailingPE"`
	ForwardPE              float64 `json:"ForwardPE"`
	PriceSalesTTM          float64 `json:"PriceSalesTTM"`
	PriceBookMRQ           float64 `json:"PriceBookMRQ"`
	EnterpriseValue        float64 `json:"EnterpriseValue"`
	EnterpriseValueRevenue float64 `json:"EnterpriseValueRevenue"`
	EnterpriseValueEbitda  float64 `json:"EnterpriseValueEbitda"`
}

// Technicals contains technical analysis data.
type Technicals struct {
	Beta                  float64 `json:"Beta"`
	FiftyTwoWeekHigh      float64 `json:"52WeekHigh"`
	FiftyTwoWeekLow       float64 `json:"52WeekLow"`
	FiftyDayMA            float64 `json:"50DayMA"`
	TwoHundredDayMA       float64 `json:"200DayMA"`
	SharesShort           int64   `json:"SharesShort"`
	SharesShortPriorMonth int64   `json:"SharesShortPriorMonth"`
	ShortRatio            float64 `json:"ShortRatio"`
	ShortPercent          float64 `json:"ShortPercent"`
}

// SplitsDividends contains splits and dividend information.
type SplitsDividends struct {
	ForwardAnnualDividendRate  float64 `json:"ForwardAnnualDividendRate"`
	ForwardAnnualDividendYield float64 `json:"ForwardAnnualDividendYield"`
	PayoutRatio                float64 `json:"PayoutRatio"`
	DividendDate               string  `json:"DividendDate"`
	ExDividendDate             string  `json:"ExDividendDate"`
	LastSplitFactor            string  `json:"LastSplitFactor"`
	LastSplitDate              string  `json:"LastSplitDate"`
}

// AnalystRatings contains analyst ratings data.
type AnalystRatings struct {
	Rating      float64 `json:"Rating"`
	TargetPrice float64 `json:"TargetPrice"`
	StrongBuy   int     `json:"StrongBuy"`
	Buy         int     `json:"Buy"`
	Hold        int     `json:"Hold"`
	Sell        int     `json:"Sell"`
	StrongSell  int     `json:"StrongSell"`
}

// Holders contains shareholder information.
type Holders struct {
	Institutions []InstitutionHolder `json:"Institutions"`
	Funds        []FundHolder        `json:"Funds"`
}

// InstitutionHolder represents an institutional holder.
type InstitutionHolder struct {
	Name          string  `json:"name"`
	Date          string  `json:"date"`
	TotalShares   int64   `json:"totalShares"`
	TotalAssets   float64 `json:"totalAssets"`
	CurrentShares int64   `json:"currentShares"`
	Change        int64   `json:"change"`
	ChangePercent float64 `json:"change_p"`
}

// FundHolder represents a fund holder.
type FundHolder struct {
	Name          string  `json:"name"`
	Date          string  `json:"date"`
	TotalShares   int64   `json:"totalShares"`
	TotalAssets   float64 `json:"totalAssets"`
	CurrentShares int64   `json:"currentShares"`
	Change        int64   `json:"change"`
	ChangePercent float64 `json:"change_p"`
}

// ESGScores contains ESG (Environmental, Social, Governance) scores.
type ESGScores struct {
	RatingDate       string  `json:"ratingDate"`
	TotalEsg         float64 `json:"totalEsg"`
	EnvironmentScore float64 `json:"environmentScore"`
	SocialScore      float64 `json:"socialScore"`
	GovernanceScore  float64 `json:"governanceScore"`
	ControversyLevel int     `json:"controversyLevel"`
}

// OutstandingShares contains outstanding shares information.
type OutstandingShares struct {
	Annual    []SharesEntry `json:"annual"`
	Quarterly []SharesEntry `json:"quarterly"`
}

// SharesEntry represents a single entry in outstanding shares.
type SharesEntry struct {
	Date          string  `json:"date"`
	DateFormatted string  `json:"dateFormatted"`
	SharesMln     float64 `json:"sharesMln"`
	Shares        int64   `json:"shares"`
}

// Earnings contains earnings data.
type Earnings struct {
	History []EarningsHistoryEntry `json:"History"`
	Trend   []EarningsTrendEntry   `json:"Trend"`
	Annual  []EarningsAnnualEntry  `json:"Annual"`
}

// EarningsHistoryEntry represents a single earnings history entry.
type EarningsHistoryEntry struct {
	ReportDate        string  `json:"reportDate"`
	Date              string  `json:"date"`
	BeforeAfterMarket string  `json:"beforeAfterMarket"`
	Currency          string  `json:"currency"`
	EPSActual         float64 `json:"epsActual"`
	EPSEstimate       float64 `json:"epsEstimate"`
	EPSDifference     float64 `json:"epsDifference"`
	SurprisePercent   float64 `json:"surprisePercent"`
}

// EarningsTrendEntry represents a single earnings trend entry.
type EarningsTrendEntry struct {
	Date                             string  `json:"date"`
	Period                           string  `json:"period"`
	Growth                           float64 `json:"growth"`
	EarningsEstimateAvg              float64 `json:"earningsEstimateAvg"`
	EarningsEstimateLow              float64 `json:"earningsEstimateLow"`
	EarningsEstimateHigh             float64 `json:"earningsEstimateHigh"`
	EarningsEstimateNumberOfAnalysts int     `json:"earningsEstimateNumberOfAnalysts"`
	RevenueEstimateAvg               float64 `json:"revenueEstimateAvg"`
	RevenueEstimateLow               float64 `json:"revenueEstimateLow"`
	RevenueEstimateHigh              float64 `json:"revenueEstimateHigh"`
	RevenueEstimateNumberOfAnalysts  int     `json:"revenueEstimateNumberOfAnalysts"`
	EPSTrendCurrent                  float64 `json:"epsTrendCurrent"`
	EPSTrend7DaysAgo                 float64 `json:"epsTrend7daysAgo"`
	EPSTrend30DaysAgo                float64 `json:"epsTrend30daysAgo"`
	EPSTrend60DaysAgo                float64 `json:"epsTrend60daysAgo"`
	EPSTrend90DaysAgo                float64 `json:"epsTrend90daysAgo"`
	EPSRevisionsUpLast7Days          int     `json:"epsRevisionsUpLast7days"`
	EPSRevisionsUpLast30Days         int     `json:"epsRevisionsUpLast30days"`
	EPSRevisionsDownLast30Days       int     `json:"epsRevisionsDownLast30days"`
}

// EarningsAnnualEntry represents annual earnings.
type EarningsAnnualEntry struct {
	Date      string  `json:"date"`
	EPSActual float64 `json:"epsActual"`
}

// Financials contains financial statements.
type Financials struct {
	BalanceSheet    *FinancialStatement `json:"Balance_Sheet"`
	CashFlow        *FinancialStatement `json:"Cash_Flow"`
	IncomeStatement *FinancialStatement `json:"Income_Statement"`
}

// FinancialStatement represents a financial statement with quarterly and yearly data.
type FinancialStatement struct {
	Currency  string                            `json:"currency"`
	Quarterly map[string]map[string]interface{} `json:"quarterly"`
	Yearly    map[string]map[string]interface{} `json:"yearly"`
}
