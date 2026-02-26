package apiclient

import "time"

// Content types

type Content struct {
	ID              string                 `json:"id"`
	TemplateID      string                 `json:"template_id"`
	TemplateName    string                 `json:"template_name"`
	Title           string                 `json:"title"`
	Slug            string                 `json:"slug"`
	FolderID        *string                `json:"folder_id,omitempty"`
	FolderPath      string                 `json:"folder_path"`
	FullPath        string                 `json:"full_path"`
	Category        string                 `json:"category"`
	MetaDescription string                 `json:"meta_description"`
	OGImage         string                 `json:"og_image"`
	Data            map[string]interface{} `json:"data"`
	Published       bool                   `json:"published"`
	PublishedAt     *time.Time             `json:"published_at,omitempty"`
	UseHeader       bool                   `json:"use_header"`
	UseFooter       bool                   `json:"use_footer"`
	UseTheme        bool                   `json:"use_theme"`
	RawMode         bool                   `json:"raw_mode"`
	Deleted         bool                   `json:"deleted"`
	DeletedAt       *time.Time             `json:"deleted_at,omitempty"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
}

type ContentVersion struct {
	ID              string                 `json:"id"`
	ContentID       string                 `json:"content_id"`
	Version         int                    `json:"version"`
	Comment         string                 `json:"comment,omitempty"`
	TemplateID      string                 `json:"template_id"`
	TemplateName    string                 `json:"template_name"`
	Title           string                 `json:"title"`
	Slug            string                 `json:"slug"`
	FolderID        *string                `json:"folder_id,omitempty"`
	FolderPath      string                 `json:"folder_path"`
	FullPath        string                 `json:"full_path"`
	Category        string                 `json:"category"`
	MetaDescription string                 `json:"meta_description"`
	OGImage         string                 `json:"og_image"`
	Data            map[string]interface{} `json:"data"`
	Published       bool                   `json:"published"`
	PublishedAt     *time.Time             `json:"published_at,omitempty"`
	UseHeader       bool                   `json:"use_header"`
	UseFooter       bool                   `json:"use_footer"`
	UseTheme        bool                   `json:"use_theme"`
	RawMode         bool                   `json:"raw_mode"`
	CreatedAt       time.Time              `json:"created_at"`
}

type CreateContentRequest struct {
	TemplateID      string                 `json:"template_id"`
	Title           string                 `json:"title"`
	Slug            string                 `json:"slug"`
	FolderPath      string                 `json:"folder_path,omitempty"`
	Category        string                 `json:"category,omitempty"`
	MetaDescription string                 `json:"meta_description,omitempty"`
	OGImage         string                 `json:"og_image,omitempty"`
	Data            map[string]interface{} `json:"data"`
	Published       bool                   `json:"published,omitempty"`
	UseHeader       bool                   `json:"use_header,omitempty"`
	UseFooter       bool                   `json:"use_footer,omitempty"`
	UseTheme        bool                   `json:"use_theme,omitempty"`
	RawMode         bool                   `json:"raw_mode,omitempty"`
	VersionComment  string                 `json:"version_comment,omitempty"`
}

type UpdateContentRequest map[string]interface{}

// Template types

type TemplateField struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Placeholder string `json:"placeholder,omitempty"`
	Options     string `json:"options,omitempty"`
	Default     string `json:"default,omitempty"`
}

type Template struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Slug        string          `json:"slug"`
	Description string          `json:"description"`
	Fields      []TemplateField `json:"fields"`
	HTMLLayout  string          `json:"html_layout"`
	Category    string          `json:"category"`
	IsSystem    bool            `json:"is_system"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

type CreateTemplateRequest struct {
	Name        string          `json:"name"`
	Slug        string          `json:"slug"`
	Description string          `json:"description,omitempty"`
	Fields      []TemplateField `json:"fields"`
	HTMLLayout  string          `json:"html_layout"`
	Category    string          `json:"category,omitempty"`
}

type UpdateTemplateRequest map[string]interface{}

// Asset types

type AssetSummary struct {
	ID          string `json:"id"`
	Filename    string `json:"filename"`
	Folder      string `json:"folder"`
	FullPath    string `json:"full_path"`
	ServePath   string `json:"serve_path"`
	MimeType    string `json:"mime_type"`
	Size        int64  `json:"size"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
}

type UploadAssetRequest struct {
	Filename    string `json:"filename"`
	ServePath   string `json:"serve_path"`
	DataBase64  string `json:"data_base64"`
	Description string `json:"description,omitempty"`
}

// Theme types

type ThemeSettings struct {
	PrimaryColor    string `json:"primary_color"`
	SecondaryColor  string `json:"secondary_color"`
	AccentColor     string `json:"accent_color"`
	BackgroundColor string `json:"background_color"`
	TextColor       string `json:"text_color"`
	FontFamily      string `json:"font_family"`
	HeadingFont     string `json:"heading_font"`
	BorderRadius    string `json:"border_radius"`
	CustomCSS       string `json:"custom_css"`
	SiteName        string `json:"site_name"`
	SiteTagline     string `json:"site_tagline"`
	LogoURL         string `json:"logo_url"`
	HeadHTML        string `json:"head_html"`
	HeaderHTML      string `json:"header_html"`
	FooterHTML      string `json:"footer_html"`
}

type ThemeVersion struct {
	ID              string    `json:"id"`
	Version         int       `json:"version"`
	Comment         string    `json:"comment,omitempty"`
	PrimaryColor    string    `json:"primary_color"`
	SecondaryColor  string    `json:"secondary_color"`
	AccentColor     string    `json:"accent_color"`
	BackgroundColor string    `json:"background_color"`
	TextColor       string    `json:"text_color"`
	FontFamily      string    `json:"font_family"`
	HeadingFont     string    `json:"heading_font"`
	BorderRadius    string    `json:"border_radius"`
	CustomCSS       string    `json:"custom_css"`
	SiteName        string    `json:"site_name"`
	SiteTagline     string    `json:"site_tagline"`
	LogoURL         string    `json:"logo_url"`
	HeadHTML        string    `json:"head_html"`
	HeaderHTML      string    `json:"header_html"`
	FooterHTML      string    `json:"footer_html"`
	CreatedAt       time.Time `json:"created_at"`
}

// Site Config

type SiteConfig struct {
	TitleTemplate        string `json:"title_template"`
	TitleTemplateNoTitle string `json:"title_template_no_title"`
}

// Redirect

type Redirect struct {
	ID          string    `json:"id"`
	FromPath    string    `json:"from_path"`
	ToPath      string    `json:"to_path"`
	StatusCode  int       `json:"status_code"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CreateRedirectRequest struct {
	FromPath    string `json:"from_path"`
	ToPath      string `json:"to_path"`
	StatusCode  int    `json:"status_code,omitempty"`
	Description string `json:"description,omitempty"`
}

// Folder

type Folder struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	ParentID  *string   `json:"parent_id,omitempty"`
	Path      string    `json:"path"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateFolderRequest struct {
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	ParentID string `json:"parent_id,omitempty"`
}

// Collection

type Collection struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Slug         string    `json:"slug"`
	Description  string    `json:"description"`
	Category     string    `json:"category"`
	SortField    string    `json:"sort_field"`
	SortOrder    string    `json:"sort_order"`
	ItemTemplate string    `json:"item_template"`
	PageTemplate string    `json:"page_template"`
	ItemsPerPage int       `json:"items_per_page"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Search

type SearchMatch struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	FullPath     string   `json:"full_path"`
	TemplateName string   `json:"template_name"`
	Published    bool     `json:"published"`
	MatchedIn    []string `json:"matched_in"`
}

type SearchResult struct {
	Query      string        `json:"query"`
	SearchType string        `json:"search_type"`
	Total      int           `json:"total"`
	Matches    []SearchMatch `json:"matches"`
}

type SearchReplaceMatch struct {
	ID            string         `json:"id"`
	Title         string         `json:"title"`
	FullPath      string         `json:"full_path"`
	Published     bool           `json:"published"`
	MatchCount    int            `json:"match_count"`
	FieldMatches  map[string]int `json:"field_matches,omitempty"`
	FieldsUpdated []string       `json:"fields_updated,omitempty"`
}

type SearchReplaceResult struct {
	Success           bool                 `json:"success,omitempty"`
	Search            string               `json:"search"`
	Replace           string               `json:"replace"`
	TotalMatches      int                  `json:"total_matches,omitempty"`
	TotalReplacements int                  `json:"total_replacements,omitempty"`
	AffectedPages     int                  `json:"affected_pages,omitempty"`
	PagesUpdated      int                  `json:"pages_updated,omitempty"`
	Matches           []SearchReplaceMatch `json:"matches,omitempty"`
	UpdatedPages      []SearchReplaceMatch `json:"updated_pages,omitempty"`
}

// API Key

type APIKey struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Prefix      string     `json:"prefix"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

type APIKeyCreated struct {
	Key         string    `json:"key"`
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Prefix      string    `json:"prefix"`
	CreatedAt   time.Time `json:"created_at"`
}
