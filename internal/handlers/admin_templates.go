package handlers

var adminTemplates = map[string]string{
	"login": `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Login - LightCMS</title>
    <link rel="icon" type="image/x-icon" href="/static/images/favicon.ico">
    <link rel="icon" type="image/png" sizes="16x16" href="/static/images/favicon-16x16.png">
    <link rel="icon" type="image/png" sizes="32x32" href="/static/images/favicon-32x32.png">
    <link rel="icon" type="image/png" sizes="48x48" href="/static/images/favicon-48x48.png">
    <link rel="apple-touch-icon" sizes="180x180" href="/static/images/apple-touch-icon.png">
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&family=Space+Grotesk:wght@400;500;600;700&display=swap" rel="stylesheet">
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: 'Inter', system-ui, sans-serif;
            background: linear-gradient(135deg, #0f172a 0%, #1e1b4b 50%, #0f172a 100%);
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 1rem;
        }
        .login-card {
            background: rgba(30, 27, 75, 0.5);
            backdrop-filter: blur(20px);
            border: 1px solid rgba(99, 102, 241, 0.2);
            border-radius: 24px;
            padding: 3rem;
            width: 100%;
            max-width: 420px;
            box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.5);
        }
        .logo {
            text-align: center;
            margin-bottom: 0.5rem;
        }
        .logo img {
            height: 48px;
            width: auto;
        }
        .subtitle {
            color: #94a3b8;
            text-align: center;
            margin-bottom: 2rem;
            font-size: 0.9rem;
        }
        .error {
            background: rgba(239, 68, 68, 0.1);
            border: 1px solid rgba(239, 68, 68, 0.3);
            color: #f87171;
            padding: 0.75rem 1rem;
            border-radius: 8px;
            margin-bottom: 1.5rem;
            font-size: 0.9rem;
        }
        label {
            display: block;
            color: #e2e8f0;
            margin-bottom: 0.5rem;
            font-weight: 500;
        }
        input[type="password"] {
            width: 100%;
            padding: 0.875rem 1rem;
            background: rgba(15, 23, 42, 0.5);
            border: 1px solid rgba(99, 102, 241, 0.3);
            border-radius: 12px;
            color: #f1f5f9;
            font-size: 1rem;
            transition: all 0.2s;
        }
        input[type="password"]:focus {
            outline: none;
            border-color: #6366f1;
            box-shadow: 0 0 0 3px rgba(99, 102, 241, 0.2);
        }
        button {
            width: 100%;
            padding: 0.875rem;
            background: linear-gradient(135deg, #6366f1, #8b5cf6);
            border: none;
            border-radius: 12px;
            color: white;
            font-size: 1rem;
            font-weight: 600;
            cursor: pointer;
            margin-top: 1.5rem;
            transition: all 0.2s;
        }
        button:hover {
            transform: translateY(-2px);
            box-shadow: 0 10px 20px -10px rgba(99, 102, 241, 0.5);
        }
    </style>
</head>
<body>
    <div class="login-card">
        <h1 class="logo"><img src="/static/images/lightcms-logo.png" alt="LightCMS"></h1>
        <p class="subtitle">Content Management System</p>
        {{if .Error}}<div class="error">{{.Error}}</div>{{end}}
        <form method="POST" action="/cm/login" autocomplete="off">
            {{.CSRFField}}
            <label for="password">Admin Password</label>
            <input type="password" id="password" name="password" placeholder="Enter password" required autofocus autocomplete="current-password" {{if .RateLimited}}disabled{{end}}>
            <button type="submit" {{if .RateLimited}}disabled style="opacity: 0.5; cursor: not-allowed;"{{end}}>Sign In</button>
        </form>
    </div>
</body>
</html>`,

	"dashboard": adminLayoutStart + `
        <div class="dashboard">
            {{if .IsDefaultPassword}}
            <div class="security-alert">
                <div class="alert-icon">⚠️</div>
                <div class="alert-content">
                    <strong>Security Warning:</strong> Your site is using the default password.
                    <a href="/cm/security">Change your password now</a> to secure your admin panel.
                </div>
            </div>
            {{end}}
            <h1>Dashboard</h1>
            <div class="stats-grid">
                <div class="stat-card">
                    <div class="stat-icon">📄</div>
                    <div class="stat-info">
                        <span class="stat-value">{{.ContentCount}}</span>
                        <span class="stat-label">Content Items</span>
                    </div>
                </div>
                <div class="stat-card">
                    <div class="stat-icon">📋</div>
                    <div class="stat-info">
                        <span class="stat-value">{{.TemplateCount}}</span>
                        <span class="stat-label">Templates</span>
                    </div>
                </div>
                <div class="stat-card">
                    <div class="stat-icon">📁</div>
                    <div class="stat-info">
                        <span class="stat-value">{{.CollectionCount}}</span>
                        <span class="stat-label">Collections</span>
                    </div>
                </div>
            </div>

            <div class="quick-actions">
                <h2>Quick Actions</h2>
                <div class="action-buttons">
                    <a href="/cm/content/new" class="btn btn-primary">New Content</a>
                    <a href="/cm/templates/new" class="btn btn-secondary">New Template</a>
                    <a href="/" target="_blank" class="btn btn-outline">View Site</a>
                </div>
            </div>

            {{if .RecentContent}}
            <div class="recent-content">
                <h2>Recent Content</h2>
                <div class="content-list">
                    {{range .RecentContent}}
                    <div class="content-item">
                        <span class="content-info">
                            <span class="content-title">{{.Title}}</span>
                            <span class="content-slug">/{{if .Slug}}{{.Slug}}{{else}}(homepage){{end}}</span>
                        </span>
                        <span class="content-template">{{.TemplateName}}</span>
                        <span class="content-status {{if .Published}}published{{else}}draft{{end}}">
                            {{if .Published}}Published{{else}}Draft{{end}}
                        </span>
                        <span class="content-actions">
                            <a href="/{{.Slug}}" target="_blank" class="btn btn-sm btn-outline">View</a>
                            <a href="/cm/content/{{.ID.Hex}}" class="btn btn-sm">Edit</a>
                        </span>
                    </div>
                    {{end}}
                </div>
            </div>
            {{end}}
        </div>
    ` + adminLayoutEnd,

	"templates_list": adminLayoutStart + `
        <div class="page-header">
            <h1>Templates</h1>
            <a href="/cm/templates/new" class="btn btn-primary">New Template</a>
        </div>
        <div class="table-container">
            <table>
                <thead>
                    <tr>
                        <th>Name</th>
                        <th>Category</th>
                        <th>Fields</th>
                        <th>System</th>
                        <th>Actions</th>
                    </tr>
                </thead>
                <tbody>
                    {{range .Templates}}
                    <tr>
                        <td><strong>{{.Name}}</strong><br><small>{{.Description}}</small></td>
                        <td>{{.Category}}</td>
                        <td>{{len .Fields}}</td>
                        <td>{{if .IsSystem}}Yes{{else}}No{{end}}</td>
                        <td class="actions">
                            <a href="/cm/templates/{{.ID.Hex}}" class="btn btn-sm">Edit</a>
                            {{if not .IsSystem}}
                            <form method="POST" action="/cm/templates/{{.ID.Hex}}/delete" onsubmit="return confirmDelete(this, 'Are you sure you want to delete this template?')">
            {{$.CSRFField}}
                                <button type="submit" class="btn btn-sm btn-danger">Delete</button>
                            </form>
                            {{end}}
                        </td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>
    ` + adminLayoutEnd,

	"template_form": adminLayoutStart + `
        <div class="page-header">
            <h1>{{if .IsNew}}New Template{{else}}Edit Template{{end}}</h1>
        </div>
        {{if .Error}}<div class="error-message">{{.Error}}</div>{{end}}
        <form method="POST" class="form-card">
            {{.CSRFField}}
            <div class="form-group">
                <label for="name">Template Name</label>
                <input type="text" id="name" name="name" value="{{if .Template}}{{.Template.Name}}{{end}}" required>
            </div>
            <div class="form-group">
                <label for="description">Description</label>
                <textarea id="description" name="description" rows="2">{{if .Template}}{{.Template.Description}}{{end}}</textarea>
            </div>
            <div class="form-group">
                <label for="category">Category</label>
                <input type="text" id="category" name="category" value="{{if .Template}}{{.Template.Category}}{{end}}" placeholder="e.g., blog, press, pages">
            </div>

            <div class="form-section">
                <h3>Fields</h3>
                <div id="fields-container">
                    {{if .Template}}{{range .Template.Fields}}
                    <div class="field-row">
                        <input type="text" name="field_name[]" value="{{.Name}}" placeholder="Field name">
                        <input type="text" name="field_label[]" value="{{.Label}}" placeholder="Label">
                        <select name="field_type[]">
                            <option value="text" {{if eq .Type "text"}}selected{{end}}>Text</option>
                            <option value="textarea" {{if eq .Type "textarea"}}selected{{end}}>Textarea</option>
                            <option value="richtext" {{if eq .Type "richtext"}}selected{{end}}>Rich Text</option>
                            <option value="rawhtml" {{if eq .Type "rawhtml"}}selected{{end}}>Raw HTML</option>
                            <option value="date" {{if eq .Type "date"}}selected{{end}}>Date</option>
                            <option value="image" {{if eq .Type "image"}}selected{{end}}>Image</option>
                            <option value="select" {{if eq .Type "select"}}selected{{end}}>Select</option>
                        </select>
                        <input type="text" name="field_placeholder[]" value="{{.Placeholder}}" placeholder="Placeholder">
                        <input type="text" name="field_options[]" value="{{.Options}}" placeholder="Options (comma-sep)">
                        <label class="checkbox-label"><input type="checkbox" name="field_required[]" {{if .Required}}checked{{end}}> Required</label>
                        <button type="button" class="btn btn-sm btn-danger" onclick="this.parentElement.remove()">×</button>
                    </div>
                    {{end}}{{end}}
                </div>
                <button type="button" class="btn btn-secondary" onclick="addField()">Add Field</button>
            </div>

            <div class="form-group">
                <label for="html_layout">HTML Layout</label>
                <p class="help-text">Use {{.FieldName}} placeholders. Available: {{.title}}, {{.slug}}, {{.published_at}}, plus your custom fields.</p>
                <textarea id="html_layout" name="html_layout" rows="15" class="code-editor">{{if .Template}}{{.Template.HTMLLayout}}{{end}}</textarea>
            </div>

            <div class="form-actions">
                <a href="/cm/templates" class="btn btn-outline">Cancel</a>
                <button type="submit" class="btn btn-primary">{{if .IsNew}}Create Template{{else}}Update Template{{end}}</button>
            </div>
        </form>

        <script>
        function addField() {
            const container = document.getElementById('fields-container');
            const row = document.createElement('div');
            row.className = 'field-row';
            row.innerHTML = ` + "`" + `
                <input type="text" name="field_name[]" placeholder="Field name">
                <input type="text" name="field_label[]" placeholder="Label">
                <select name="field_type[]">
                    <option value="text">Text</option>
                    <option value="textarea">Textarea</option>
                    <option value="richtext">Rich Text</option>
                    <option value="rawhtml">Raw HTML</option>
                    <option value="date">Date</option>
                    <option value="image">Image</option>
                    <option value="select">Select</option>
                </select>
                <input type="text" name="field_placeholder[]" placeholder="Placeholder">
                <input type="text" name="field_options[]" placeholder="Options (comma-sep)">
                <label class="checkbox-label"><input type="checkbox" name="field_required[]"> Required</label>
                <button type="button" class="btn btn-sm btn-danger" onclick="this.parentElement.remove()">×</button>
            ` + "`" + `;
            container.appendChild(row);
        }
        </script>
    ` + adminLayoutEnd,

	"content_list": adminLayoutStart + `
        <div class="page-header">
            <h1>Content</h1>
            <a href="/cm/content/new" class="btn btn-primary">New Content</a>
        </div>

        <div class="filter-bar" style="display: flex; gap: 1rem; margin-bottom: 1.5rem; align-items: center; flex-wrap: wrap;">
            <div class="filter-group" style="display: flex; gap: 0.5rem; align-items: center;">
                <label style="font-size: 0.9rem; color: var(--muted);">Folder:</label>
                <select id="folder-filter" onchange="applyFilters()" style="padding: 0.5rem; background: var(--card-bg); border: 1px solid var(--border); border-radius: var(--radius); color: var(--text);">
                    <option value="all" {{if eq .FolderFilter "all"}}selected{{else if eq .FolderFilter ""}}selected{{end}}>All Folders</option>
                    <option value="root" {{if eq .FolderFilter "root"}}selected{{end}}>/ (Root)</option>
                    {{range .Folders}}
                    <option value="{{.Path}}" {{if eq $.FolderFilter .Path}}selected{{end}}>{{.Path}}</option>
                    {{end}}
                </select>
            </div>
            <div class="filter-group" style="display: flex; gap: 0.5rem; align-items: center;">
                <button type="button" onclick="openSearchModal()" class="btn btn-sm btn-outline" title="Search Content" style="display: flex; align-items: center; gap: 0.4rem;">
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                        <circle cx="11" cy="11" r="8"></circle>
                        <path d="M21 21l-4.35-4.35"></path>
                    </svg>
                    Search
                </button>
            </div>
            <div id="search-indicator" class="filter-group" style="display: none; gap: 0.5rem; align-items: center;">
                <span id="search-query-display" style="font-size: 0.9rem; color: var(--primary);"></span>
                <button type="button" onclick="clearSearch()" class="btn btn-sm btn-outline">Show All</button>
            </div>
        </div>

        <div class="table-container">
            <table>
                <thead>
                    <tr>
                        <th>Title</th>
                        <th>Template</th>
                        <th>Path</th>
                        <th>Status</th>
                        <th>Updated</th>
                        <th>Actions</th>
                    </tr>
                </thead>
                <tbody>
                    {{range .Content}}
                    <tr{{if .Deleted}} style="opacity: 0.7;"{{end}}>
                        <td><strong>{{.Title}}</strong></td>
                        <td>{{.TemplateName}}</td>
                        <td><code>{{if .Deleted}}(deleted){{else if .FullPath}}{{.FullPath}}{{else}}/{{.Slug}}{{end}}</code></td>
                        <td>
                            {{if .Deleted}}
                            <span class="status-badge" style="background: var(--danger);">Deleted</span>
                            {{else}}
                            <span class="status-badge {{if .Published}}published{{else}}draft{{end}}">{{if .Published}}Published{{else}}Draft{{end}}</span>
                            {{end}}
                        </td>
                        <td>{{.UpdatedAt.Format "Jan 2, 2006"}}</td>
                        <td class="actions">
                            {{if .Deleted}}
                            <a href="/cm/content/{{.ID.Hex}}/versions/latest/view" target="_blank" class="btn btn-sm btn-outline">View</a>
                            <a href="/cm/content/{{.ID.Hex}}" class="btn btn-sm">Edit</a>
                            <form method="POST" action="/cm/content/{{.ID.Hex}}/undelete" style="display:inline">
            {{$.CSRFField}}
                                <button type="submit" class="btn btn-sm btn-primary">Restore</button>
                            </form>
                            {{else}}
                            <a href="{{if .FullPath}}{{.FullPath}}{{else}}/{{.Slug}}{{end}}" target="_blank" class="btn btn-sm btn-outline">View</a>
                            <a href="/cm/content/{{.ID.Hex}}" class="btn btn-sm">Edit</a>
                            <form method="POST" action="/cm/content/{{.ID.Hex}}/regenerate" style="display:inline">
            {{$.CSRFField}}
                                <button type="submit" class="btn btn-sm btn-secondary" title="Regenerate static file">↻</button>
                            </form>
                            <form method="POST" action="/cm/content/{{.ID.Hex}}/delete" onsubmit="return confirmDelete(this, 'Are you sure you want to delete this content?')">
            {{$.CSRFField}}
                                <button type="submit" class="btn btn-sm btn-danger">Delete</button>
                            </form>
                            {{end}}
                        </td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>
        <!-- Search Modal -->
        <div id="search-modal" class="modal-overlay" style="display: none;">
            <div class="modal-content" style="max-width: 500px;">
                <div class="modal-header">
                    <h2 id="search-modal-title">Search Content</h2>
                    <button type="button" onclick="closeSearchModal()" class="modal-close">&times;</button>
                </div>
                <div class="modal-body">
                    <div class="form-group" style="margin-bottom: 1rem;">
                        <label style="display: block; margin-bottom: 0.5rem; font-weight: 500;">Search Query</label>
                        <input type="text" id="search-query" placeholder="Leave empty to show all..."
                               style="width: 100%; padding: 0.75rem; background: var(--card-bg); border: 1px solid var(--border); border-radius: var(--radius); color: var(--text); font-size: 1rem;"
                               onkeydown="if(event.key==='Enter') { if(document.getElementById('enable-replace').checked) { previewReplace(); } else { performSearch(); } }">
                    </div>
                    <div class="form-group" style="margin-bottom: 1rem;">
                        <label style="display: block; margin-bottom: 0.5rem; font-weight: 500;">Search In</label>
                        <div style="display: flex; gap: 1rem;">
                            <label style="display: flex; align-items: center; gap: 0.5rem; cursor: pointer;">
                                <input type="radio" name="search-type" value="name" checked style="accent-color: var(--primary);" onchange="updateSearchMode()">
                                <span>Title only</span>
                            </label>
                            <label style="display: flex; align-items: center; gap: 0.5rem; cursor: pointer;">
                                <input type="radio" name="search-type" value="fulltext" style="accent-color: var(--primary);" onchange="updateSearchMode()">
                                <span>Full text</span>
                            </label>
                        </div>
                    </div>
                    <div class="form-group" style="margin-bottom: 1rem;">
                        <label style="display: flex; align-items: center; gap: 0.5rem; cursor: pointer;">
                            <input type="checkbox" id="include-deleted" style="accent-color: var(--danger); width: 16px; height: 16px;">
                            <span style="color: var(--danger);">Include deleted content</span>
                        </label>
                    </div>
                    <div class="form-group" style="margin-bottom: 1rem; padding-top: 0.5rem; border-top: 1px solid var(--border);">
                        <label style="display: flex; align-items: center; gap: 0.5rem; cursor: pointer;">
                            <input type="checkbox" id="enable-replace" style="accent-color: var(--warning); width: 16px; height: 16px;" onchange="toggleReplaceMode()">
                            <span style="color: var(--warning);">Search and Replace (full text only)</span>
                        </label>
                    </div>
                    <div id="replace-field" class="form-group" style="margin-bottom: 1.5rem; display: none;">
                        <label style="display: block; margin-bottom: 0.5rem; font-weight: 500;">Replace With</label>
                        <input type="text" id="replace-query" placeholder="Replacement text..."
                               style="width: 100%; padding: 0.75rem; background: var(--card-bg); border: 1px solid var(--border); border-radius: var(--radius); color: var(--text); font-size: 1rem;">
                    </div>
                    <button type="button" id="search-btn" onclick="performSearch()" class="btn btn-primary" style="width: 100%;">Search</button>
                    <button type="button" id="preview-replace-btn" onclick="previewReplace()" class="btn btn-warning" style="width: 100%; display: none;">Preview Replacements</button>
                </div>
            </div>
        </div>

        <!-- Replace Preview Modal -->
        <div id="replace-preview-modal" class="modal-overlay" style="display: none;">
            <div class="modal-content" style="max-width: 900px;">
                <div class="modal-header" style="background: rgba(234, 179, 8, 0.1); border-bottom-color: var(--warning);">
                    <h2 style="color: var(--warning);">Search and Replace Preview</h2>
                    <button type="button" onclick="closeReplacePreview()" class="modal-close">&times;</button>
                </div>
                <div class="modal-body">
                    <div class="replace-warning" style="background: rgba(239, 68, 68, 0.1); border: 1px solid rgba(239, 68, 68, 0.3); border-radius: var(--radius); padding: 1rem; margin-bottom: 1.5rem;">
                        <strong style="color: var(--danger);">Warning:</strong> Site-wide replace can be destructive. Please carefully review all changes below before proceeding. Each affected page will have a version saved before changes are made.
                    </div>
                    <div id="replace-summary" style="margin-bottom: 1rem; padding: 0.75rem; background: var(--card-bg); border-radius: var(--radius);">
                        <!-- Summary will be inserted here -->
                    </div>
                    <div id="replace-preview-list" style="max-height: 400px; overflow-y: auto; margin-bottom: 1.5rem;">
                        <!-- Preview items will be inserted here -->
                    </div>
                    <div style="display: flex; gap: 1rem;">
                        <button type="button" onclick="closeReplacePreview()" class="btn btn-outline" style="flex: 1;">Cancel</button>
                        <button type="button" id="execute-replace-btn" onclick="executeReplace()" class="btn btn-danger" style="flex: 1;">Accept Replacements</button>
                    </div>
                </div>
            </div>
        </div>

        <style>
            code {
                font-family: 'JetBrains Mono', monospace;
                background: rgba(99, 102, 241, 0.1);
                padding: 0.25rem 0.5rem;
                border-radius: 4px;
                font-size: 0.85rem;
            }
            .modal-overlay {
                position: fixed;
                top: 0;
                left: 0;
                right: 0;
                bottom: 0;
                background: rgba(0, 0, 0, 0.7);
                display: flex;
                align-items: center;
                justify-content: center;
                z-index: 1000;
                backdrop-filter: blur(4px);
            }
            .modal-content {
                background: var(--background);
                border: 1px solid var(--border);
                border-radius: var(--radius);
                width: 90%;
                max-height: 90vh;
                overflow-y: auto;
                box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.5);
            }
            .modal-header {
                display: flex;
                justify-content: space-between;
                align-items: center;
                padding: 1rem 1.5rem;
                border-bottom: 1px solid var(--border);
            }
            .modal-header h2 {
                margin: 0;
                font-size: 1.25rem;
            }
            .modal-close {
                background: none;
                border: none;
                color: var(--muted);
                font-size: 1.5rem;
                cursor: pointer;
                padding: 0;
                line-height: 1;
            }
            .modal-close:hover {
                color: var(--text);
            }
            .modal-body {
                padding: 1.5rem;
            }
            .search-results-row {
                opacity: 0;
                animation: fadeIn 0.3s ease forwards;
            }
            @keyframes fadeIn {
                to { opacity: 1; }
            }
            .btn-warning {
                background: linear-gradient(135deg, #eab308, #ca8a04);
                color: #000;
            }
            .btn-warning:hover {
                box-shadow: 0 10px 20px -10px rgba(234, 179, 8, 0.5);
            }
            .replace-preview-item {
                background: var(--card-bg);
                border: 1px solid var(--border);
                border-radius: var(--radius);
                padding: 1rem;
                margin-bottom: 0.75rem;
            }
            .replace-preview-item h4 {
                margin: 0 0 0.5rem 0;
                font-size: 1rem;
            }
            .replace-preview-item .path {
                font-size: 0.85rem;
                color: var(--muted);
                margin-bottom: 0.75rem;
            }
            .replace-excerpt {
                font-family: 'JetBrains Mono', monospace;
                font-size: 0.85rem;
                background: rgba(15, 23, 42, 0.5);
                padding: 0.75rem;
                border-radius: 4px;
                overflow-x: auto;
                white-space: pre-wrap;
                word-break: break-word;
            }
            .replace-old {
                background: rgba(239, 68, 68, 0.2);
                color: #fca5a5;
                text-decoration: line-through;
            }
            .replace-new {
                background: rgba(34, 197, 94, 0.2);
                color: #86efac;
            }
        </style>
        <script>
            var csrfToken = '{{.CSRFToken}}';
            var originalTableBody = null;
            var isSearchActive = false;

            function applyFilters() {
                var folder = document.getElementById('folder-filter').value;
                var params = new URLSearchParams(window.location.search);
                if (folder && folder !== 'all') {
                    params.set('folder', folder);
                } else {
                    params.delete('folder');
                }
                window.location.search = params.toString();
            }

            function openSearchModal() {
                document.getElementById('search-modal').style.display = 'flex';
                document.getElementById('search-query').focus();
            }

            function closeSearchModal() {
                document.getElementById('search-modal').style.display = 'none';
            }

            function performSearch() {
                var query = document.getElementById('search-query').value.trim();
                var searchType = document.querySelector('input[name="search-type"]:checked').value;
                var includeDeleted = document.getElementById('include-deleted').checked;

                // Store original table body if not already stored
                if (!originalTableBody) {
                    originalTableBody = document.querySelector('tbody').innerHTML;
                }

                // Show loading state
                var tbody = document.querySelector('tbody');
                tbody.innerHTML = '<tr><td colspan="6" style="text-align: center; padding: 2rem; color: var(--muted);">Searching...</td></tr>';

                var url = '/api/content/search?type=' + searchType;
                if (query) {
                    url += '&q=' + encodeURIComponent(query);
                }
                if (includeDeleted) {
                    url += '&deleted=true';
                }

                fetch(url)
                    .then(function(response) { return response.json(); })
                    .then(function(results) {
                        closeSearchModal();
                        displaySearchResults(results, query, includeDeleted);
                    })
                    .catch(function(err) {
                        tbody.innerHTML = '<tr><td colspan="6" style="text-align: center; padding: 2rem; color: var(--danger);">Search failed: ' + err.message + '</td></tr>';
                    });
            }

            function displaySearchResults(results, query, includeDeleted) {
                var tbody = document.querySelector('tbody');
                isSearchActive = true;

                // Show search indicator with appropriate message
                document.getElementById('search-indicator').style.display = 'flex';
                var indicatorText = '';
                if (query) {
                    indicatorText = 'Search: <strong>' + escapeHtml(query) + '</strong>';
                } else {
                    indicatorText = '<strong>All content</strong>';
                }
                if (includeDeleted) {
                    indicatorText += ' <span style="color: var(--danger);">(including deleted)</span>';
                }
                document.getElementById('search-query-display').innerHTML = indicatorText;

                if (results.length === 0) {
                    var msg = query ? 'No results found for "' + escapeHtml(query) + '"' : 'No content found';
                    tbody.innerHTML = '<tr><td colspan="6" style="text-align: center; padding: 2rem; color: var(--muted);">' + msg + '</td></tr>';
                    return;
                }

                var html = '';
                results.forEach(function(item, index) {
                    var rowStyle = item.deleted ? ' style="opacity: 0.7; animation-delay: ' + (index * 0.05) + 's;"' : ' style="animation-delay: ' + (index * 0.05) + 's;"';
                    html += '<tr class="search-results-row"' + rowStyle + '>';
                    html += '<td><strong>' + escapeHtml(item.title) + '</strong></td>';
                    html += '<td>' + escapeHtml(item.template_name) + '</td>';
                    html += '<td><code>' + (item.deleted ? '(deleted)' : escapeHtml(item.full_path)) + '</code></td>';

                    if (item.deleted) {
                        html += '<td><span class="status-badge" style="background: var(--danger);">Deleted</span></td>';
                    } else {
                        var statusClass = item.published ? 'published' : 'draft';
                        var statusText = item.published ? 'Published' : 'Draft';
                        html += '<td><span class="status-badge ' + statusClass + '">' + statusText + '</span></td>';
                    }

                    html += '<td>' + escapeHtml(item.updated_at) + '</td>';
                    html += '<td class="actions">';
                    if (item.deleted) {
                        html += '<a href="/cm/content/' + item.id + '/versions/latest/view" target="_blank" class="btn btn-sm btn-outline">View</a>';
                        html += '<a href="/cm/content/' + item.id + '" class="btn btn-sm">Edit</a>';
                        html += '<form method="POST" action="/cm/content/' + item.id + '/undelete" style="display:inline">';
                        html += '<input type="hidden" name="gorilla.csrf.Token" value="' + csrfToken + '">';
                        html += '<button type="submit" class="btn btn-sm btn-primary">Restore</button>';
                        html += '</form>';
                    } else {
                        html += '<a href="' + item.full_path + '" target="_blank" class="btn btn-sm btn-outline">View</a>';
                        html += '<a href="/cm/content/' + item.id + '" class="btn btn-sm">Edit</a>';
                    }
                    html += '</td>';
                    html += '</tr>';
                });
                tbody.innerHTML = html;
            }

            function clearSearch() {
                if (originalTableBody) {
                    document.querySelector('tbody').innerHTML = originalTableBody;
                }
                document.getElementById('search-indicator').style.display = 'none';
                document.getElementById('search-query').value = '';
                document.getElementById('include-deleted').checked = false;
                isSearchActive = false;
            }

            function escapeHtml(text) {
                var div = document.createElement('div');
                div.textContent = text;
                return div.innerHTML;
            }

            // Search and Replace functions
            var replacePreviewData = null;

            function toggleReplaceMode() {
                var enabled = document.getElementById('enable-replace').checked;
                var replaceField = document.getElementById('replace-field');
                var searchBtn = document.getElementById('search-btn');
                var previewBtn = document.getElementById('preview-replace-btn');
                var titleEl = document.getElementById('search-modal-title');

                if (enabled) {
                    replaceField.style.display = 'block';
                    searchBtn.style.display = 'none';
                    previewBtn.style.display = 'block';
                    titleEl.textContent = 'Search and Replace';
                    // Force full text mode
                    document.querySelector('input[name="search-type"][value="fulltext"]').checked = true;
                    document.getElementById('include-deleted').checked = false;
                    document.getElementById('include-deleted').disabled = true;
                } else {
                    replaceField.style.display = 'none';
                    searchBtn.style.display = 'block';
                    previewBtn.style.display = 'none';
                    titleEl.textContent = 'Search Content';
                    document.getElementById('include-deleted').disabled = false;
                }
            }

            function updateSearchMode() {
                var searchType = document.querySelector('input[name="search-type"]:checked').value;
                if (searchType !== 'fulltext' && document.getElementById('enable-replace').checked) {
                    document.getElementById('enable-replace').checked = false;
                    toggleReplaceMode();
                }
            }

            function previewReplace() {
                var searchQuery = document.getElementById('search-query').value.trim();
                var replaceQuery = document.getElementById('replace-query').value;

                if (!searchQuery) {
                    alert('Please enter a search query for replace.');
                    return;
                }

                // Show loading in preview modal
                document.getElementById('replace-preview-modal').style.display = 'flex';
                document.getElementById('replace-preview-list').innerHTML = '<div style="text-align: center; padding: 2rem; color: var(--muted);">Loading preview...</div>';
                document.getElementById('replace-summary').innerHTML = '';
                document.getElementById('execute-replace-btn').disabled = true;

                fetch('/api/content/replace-preview?search=' + encodeURIComponent(searchQuery) + '&replace=' + encodeURIComponent(replaceQuery))
                    .then(function(response) { return response.json(); })
                    .then(function(data) {
                        replacePreviewData = data;
                        displayReplacePreview(data, searchQuery, replaceQuery);
                    })
                    .catch(function(err) {
                        document.getElementById('replace-preview-list').innerHTML = '<div style="text-align: center; padding: 2rem; color: var(--danger);">Failed to load preview: ' + escapeHtml(err.message) + '</div>';
                    });
            }

            function displayReplacePreview(data, searchQuery, replaceQuery) {
                var listEl = document.getElementById('replace-preview-list');
                var summaryEl = document.getElementById('replace-summary');
                var executeBtn = document.getElementById('execute-replace-btn');

                if (!data.matches || data.matches.length === 0) {
                    summaryEl.innerHTML = '<span style="color: var(--muted);">No matches found for "' + escapeHtml(searchQuery) + '"</span>';
                    listEl.innerHTML = '';
                    executeBtn.disabled = true;
                    return;
                }

                var totalMatches = 0;
                data.matches.forEach(function(m) { totalMatches += m.match_count; });

                summaryEl.innerHTML = 'Found <strong>' + totalMatches + '</strong> occurrence(s) across <strong>' + data.matches.length + '</strong> page(s). Replacing "<code>' + escapeHtml(searchQuery) + '</code>" with "<code>' + escapeHtml(replaceQuery) + '</code>"';

                var html = '';
                data.matches.forEach(function(item) {
                    html += '<div class="replace-preview-item">';
                    html += '<h4>' + escapeHtml(item.title) + '</h4>';
                    html += '<div class="path"><code>' + escapeHtml(item.full_path) + '</code> &middot; ' + item.match_count + ' occurrence(s)</div>';

                    item.excerpts.forEach(function(excerpt) {
                        html += '<div class="replace-excerpt">' + excerpt + '</div>';
                    });

                    html += '</div>';
                });

                listEl.innerHTML = html;
                executeBtn.disabled = false;
            }

            function closeReplacePreview() {
                document.getElementById('replace-preview-modal').style.display = 'none';
                replacePreviewData = null;
            }

            function executeReplace() {
                if (!replacePreviewData || !replacePreviewData.matches || replacePreviewData.matches.length === 0) {
                    return;
                }

                var searchQuery = document.getElementById('search-query').value.trim();
                var replaceQuery = document.getElementById('replace-query').value;
                var count = replacePreviewData.matches.length;

                showConfirm('Are you sure you want to replace text in ' + count + ' page(s)?<br><br>This action will save a version of each page before making changes.', 'Confirm Replace').then(function(confirmed) {
                    if (!confirmed) return;

                    var executeBtn = document.getElementById('execute-replace-btn');
                    executeBtn.disabled = true;
                    executeBtn.textContent = 'Replacing...';

                    fetch('/api/content/replace-execute', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({
                            search: searchQuery,
                            replace: replaceQuery
                        })
                    })
                    .then(function(response) { return response.json(); })
                    .then(function(data) {
                        if (data.error) {
                            showAlert('Error: ' + data.error, 'Replace Failed');
                            executeBtn.disabled = false;
                            executeBtn.textContent = 'Accept Replacements';
                        } else {
                            closeReplacePreview();
                            closeSearchModal();
                            showAlert('Successfully updated ' + data.updated_count + ' page(s).', 'Replace Complete', function() {
                                // Reload the page to see changes
                                window.location.reload();
                            });
                        }
                    })
                    .catch(function(err) {
                        showAlert('Failed to execute replace: ' + err.message, 'Replace Failed');
                        executeBtn.disabled = false;
                        executeBtn.textContent = 'Accept Replacements';
                    });
                });
            }

            // Close modal on escape key
            document.addEventListener('keydown', function(e) {
                if (e.key === 'Escape') {
                    closeSearchModal();
                    closeReplacePreview();
                }
            });

            // Close modal on overlay click
            document.getElementById('search-modal').addEventListener('click', function(e) {
                if (e.target === this) {
                    closeSearchModal();
                }
            });
            document.getElementById('replace-preview-modal').addEventListener('click', function(e) {
                if (e.target === this) {
                    closeReplacePreview();
                }
            });
        </script>
    ` + adminLayoutEnd,

	"content_select_template": adminLayoutStart + `
        <div class="page-header">
            <h1>Create New Content</h1>
        </div>
        <p class="page-subtitle">Select a template to get started:</p>
        <div class="template-grid">
            {{range .Templates}}
            <a href="/cm/content/new/{{.ID.Hex}}" class="template-card">
                <h3>{{.Name}}</h3>
                <p>{{.Description}}</p>
                <span class="template-category">{{.Category}}</span>
            </a>
            {{end}}
        </div>
    ` + adminLayoutEnd,

	"version_diff": adminLayoutStart + `
        <div class="page-header">
            <h1>Version Comparison</h1>
            <a href="/cm/content/{{.Current.ID.Hex}}" class="btn btn-outline">← Back to Editor</a>
        </div>
        <p style="color: var(--muted); margin-bottom: 2rem;">
            Comparing <strong>Version {{.Version.Version}}</strong> (saved {{.Version.CreatedAt.Format "Jan 2, 2006 3:04 PM"}})
            with <strong>Current Version</strong>
        </p>

        <div class="diff-container">
            <!-- Title -->
            <div class="diff-section" data-field="title">
                <h3>Title <span class="diff-badge"></span></h3>
                <div class="diff-row">
                    <div class="diff-col diff-old">
                        <div class="diff-label">Version {{.Version.Version}}</div>
                        <div class="diff-content" data-old="{{.Version.Title}}">{{.Version.Title}}</div>
                    </div>
                    <div class="diff-col diff-new">
                        <div class="diff-label">Current</div>
                        <div class="diff-content" data-new="{{.Current.Title}}">{{.Current.Title}}</div>
                    </div>
                </div>
            </div>

            <!-- Slug -->
            <div class="diff-section" data-field="slug">
                <h3>Slug <span class="diff-badge"></span></h3>
                <div class="diff-row">
                    <div class="diff-col diff-old">
                        <div class="diff-label">Version {{.Version.Version}}</div>
                        <div class="diff-content" data-old="{{.Version.Slug}}"><code>{{.Version.Slug}}</code></div>
                    </div>
                    <div class="diff-col diff-new">
                        <div class="diff-label">Current</div>
                        <div class="diff-content" data-new="{{.Current.Slug}}"><code>{{.Current.Slug}}</code></div>
                    </div>
                </div>
            </div>

            <!-- Full Path -->
            <div class="diff-section" data-field="full_path">
                <h3>Full Path <span class="diff-badge"></span></h3>
                <div class="diff-row">
                    <div class="diff-col diff-old">
                        <div class="diff-label">Version {{.Version.Version}}</div>
                        <div class="diff-content" data-old="{{.Version.FullPath}}"><code>{{.Version.FullPath}}</code></div>
                    </div>
                    <div class="diff-col diff-new">
                        <div class="diff-label">Current</div>
                        <div class="diff-content" data-new="{{.Current.FullPath}}"><code>{{.Current.FullPath}}</code></div>
                    </div>
                </div>
            </div>

            <!-- Content Data Fields -->
            {{range $key, $value := .Version.Data}}
            <div class="diff-section diff-field-section" data-field="{{$key}}">
                <h3>{{$key}} <span class="diff-badge"></span></h3>
                <div class="diff-row">
                    <div class="diff-col diff-old">
                        <div class="diff-label">Version {{$.Version.Version}}</div>
                        <div class="diff-content diff-html" data-old="{{$value}}"></div>
                    </div>
                    <div class="diff-col diff-new">
                        <div class="diff-label">Current</div>
                        <div class="diff-content diff-html" data-new="{{index $.Current.Data $key}}"></div>
                    </div>
                </div>
            </div>
            {{end}}

            <!-- Settings -->
            <div class="diff-section" data-field="settings">
                <h3>Settings <span class="diff-badge"></span></h3>
                <div class="diff-row">
                    <div class="diff-col diff-old">
                        <div class="diff-label">Version {{.Version.Version}}</div>
                        <div class="diff-content" data-old="published:{{.Version.Published}};header:{{.Version.UseHeader}};footer:{{.Version.UseFooter}};theme:{{.Version.UseTheme}}">
                            <p>Published: {{if .Version.Published}}Yes{{else}}No{{end}}</p>
                            <p>Use Header: {{if .Version.UseHeader}}Yes{{else}}No{{end}}</p>
                            <p>Use Footer: {{if .Version.UseFooter}}Yes{{else}}No{{end}}</p>
                            <p>Use Theme: {{if .Version.UseTheme}}Yes{{else}}No{{end}}</p>
                        </div>
                    </div>
                    <div class="diff-col diff-new">
                        <div class="diff-label">Current</div>
                        <div class="diff-content" data-new="published:{{.Current.Published}};header:{{.Current.UseHeader}};footer:{{.Current.UseFooter}};theme:{{.Current.UseTheme}}">
                            <p>Published: {{if .Current.Published}}Yes{{else}}No{{end}}</p>
                            <p>Use Header: {{if .Current.UseHeader}}Yes{{else}}No{{end}}</p>
                            <p>Use Footer: {{if .Current.UseFooter}}Yes{{else}}No{{end}}</p>
                            <p>Use Theme: {{if .Current.UseTheme}}Yes{{else}}No{{end}}</p>
                        </div>
                    </div>
                </div>
            </div>
        </div>

        <div class="form-actions" style="margin-top: 2rem;">
            <a href="/cm/content/{{.Current.ID.Hex}}" class="btn btn-outline">Back to Editor</a>
            <a href="/cm/content/{{.Current.ID.Hex}}/versions/{{.Version.Version}}/view" target="_blank" class="btn btn-secondary">Preview Version {{.Version.Version}}</a>
            <form method="POST" action="/cm/content/{{.Current.ID.Hex}}/versions/{{.Version.Version}}/revert" style="display:inline" onsubmit="return confirmRevert(this, {{.Version.Version}})">
            {{.CSRFField}}
                <button type="submit" class="btn btn-primary">Revert to Version {{.Version.Version}}</button>
            </form>
        </div>

        <style>
            .diff-container {
                display: flex;
                flex-direction: column;
                gap: 1.5rem;
            }
            .diff-section {
                background: var(--card-bg);
                border: 1px solid var(--border);
                border-radius: var(--radius);
                padding: 1rem;
            }
            .diff-section.has-changes {
                border-color: #f59e0b;
            }
            .diff-section.no-changes {
                opacity: 0.6;
            }
            .diff-section h3 {
                margin: 0 0 1rem 0;
                font-size: 1rem;
                color: var(--accent);
                border-bottom: 1px solid var(--border);
                padding-bottom: 0.5rem;
                display: flex;
                align-items: center;
                gap: 0.75rem;
            }
            .diff-badge {
                font-size: 0.7rem;
                padding: 0.15rem 0.5rem;
                border-radius: 4px;
                text-transform: uppercase;
                font-weight: 600;
            }
            .diff-badge.changed {
                background: rgba(245, 158, 11, 0.2);
                color: #f59e0b;
            }
            .diff-badge.unchanged {
                background: rgba(107, 114, 128, 0.2);
                color: #6b7280;
            }
            .diff-row {
                display: grid;
                grid-template-columns: 1fr 1fr;
                gap: 1rem;
            }
            .diff-col {
                min-width: 0;
            }
            .diff-label {
                font-size: 0.75rem;
                text-transform: uppercase;
                color: var(--muted);
                margin-bottom: 0.5rem;
                font-weight: 600;
            }
            .diff-old .diff-label {
                color: #f59e0b;
            }
            .diff-new .diff-label {
                color: #10b981;
            }
            .diff-content {
                background: rgba(15, 23, 42, 0.5);
                border: 1px solid var(--border);
                border-radius: 4px;
                padding: 0.75rem;
                font-size: 0.9rem;
                overflow-x: auto;
                max-height: 400px;
                overflow-y: auto;
            }
            .diff-old .diff-content {
                border-left: 3px solid #f59e0b;
            }
            .diff-new .diff-content {
                border-left: 3px solid #10b981;
            }
            .diff-html {
                white-space: pre-wrap;
                word-break: break-word;
                font-family: 'JetBrains Mono', monospace;
                font-size: 0.8rem;
            }
            code {
                font-family: 'JetBrains Mono', monospace;
                background: rgba(99, 102, 241, 0.1);
                padding: 0.25rem 0.5rem;
                border-radius: 4px;
            }
            /* Inline diff highlighting */
            .diff-added {
                background: rgba(16, 185, 129, 0.3);
                color: #10b981;
                padding: 0 2px;
                border-radius: 2px;
            }
            .diff-removed {
                background: rgba(239, 68, 68, 0.3);
                color: #ef4444;
                padding: 0 2px;
                border-radius: 2px;
                text-decoration: line-through;
            }
            .diff-current {
                outline: 2px solid #6366f1;
                outline-offset: 2px;
                background: rgba(99, 102, 241, 0.2) !important;
            }
            /* Navigation controls */
            .diff-nav {
                display: flex;
                align-items: center;
                gap: 0.5rem;
                margin-bottom: 0.5rem;
                padding: 0.5rem;
                background: var(--bg-tertiary);
                border-radius: 4px;
                font-size: 0.8rem;
            }
            .diff-nav-btn {
                padding: 0.25rem 0.5rem;
                background: var(--primary);
                color: white;
                border: none;
                border-radius: 4px;
                cursor: pointer;
                font-size: 0.75rem;
                font-weight: 500;
            }
            .diff-nav-btn:hover {
                opacity: 0.9;
            }
            .diff-nav-btn:disabled {
                opacity: 0.5;
                cursor: not-allowed;
            }
            .diff-nav-count {
                color: var(--muted);
                margin-left: auto;
            }
            @media (max-width: 768px) {
                .diff-row {
                    grid-template-columns: 1fr;
                }
            }
        </style>

        <script>
        document.addEventListener('DOMContentLoaded', function() {
            // Process each diff section
            document.querySelectorAll('.diff-section').forEach(function(section) {
                var oldEl = section.querySelector('[data-old]');
                var newEl = section.querySelector('[data-new]');
                var badge = section.querySelector('.diff-badge');

                if (!oldEl || !newEl || !badge) return;

                var oldVal = oldEl.getAttribute('data-old') || '';
                var newVal = newEl.getAttribute('data-new') || '';

                if (oldVal === newVal) {
                    badge.textContent = 'Unchanged';
                    badge.className = 'diff-badge unchanged';
                    section.classList.add('no-changes');
                } else {
                    badge.textContent = 'Changed';
                    badge.className = 'diff-badge changed';
                    section.classList.add('has-changes');

                    // For content fields, show inline diff with navigation
                    if (section.classList.contains('diff-field-section')) {
                        showInlineDiff(section, oldEl, newEl, oldVal, newVal);
                    }
                }
            });

            // Line-level diff with navigation for long content
            function showInlineDiff(section, oldEl, newEl, oldText, newText) {
                // Escape HTML for display
                function escapeHtml(str) {
                    return str.replace(/&/g, '&amp;')
                              .replace(/</g, '&lt;')
                              .replace(/>/g, '&gt;')
                              .replace(/"/g, '&quot;');
                }

                // Find the differences using a simple line-based approach
                var oldLines = oldText.split('\n');
                var newLines = newText.split('\n');

                // Build a map of lines for quick lookup
                var oldLineSet = new Set(oldLines);
                var newLineSet = new Set(newLines);

                // Track change indices for pairing old/new
                var changeIdx = 0;

                // Highlight changed/removed lines in old version
                var oldHighlighted = oldLines.map(function(line, idx) {
                    var escaped = escapeHtml(line);
                    if (!newLineSet.has(line)) {
                        return '<span class="diff-removed" data-diff-idx="' + (changeIdx++) + '" data-line="' + idx + '">' + escaped + '</span>';
                    }
                    return '<span data-line="' + idx + '">' + escaped + '</span>';
                }).join('\n');

                // Reset for new side - track which change we're on
                var newChangeIdx = 0;

                // Highlight changed/added lines in new version
                var newHighlighted = newLines.map(function(line, idx) {
                    var escaped = escapeHtml(line);
                    if (!oldLineSet.has(line)) {
                        return '<span class="diff-added" data-diff-idx="' + (newChangeIdx++) + '" data-line="' + idx + '">' + escaped + '</span>';
                    }
                    return '<span data-line="' + idx + '">' + escaped + '</span>';
                }).join('\n');

                oldEl.innerHTML = oldHighlighted;
                newEl.innerHTML = newHighlighted;

                // Get all change elements from both sides
                var oldChanges = oldEl.querySelectorAll('.diff-removed');
                var newChanges = newEl.querySelectorAll('.diff-added');
                var totalChanges = Math.max(oldChanges.length, newChanges.length);

                // Only add navigation if there are changes
                if (totalChanges > 0) {
                    var currentIndex = 0;

                    // Create navigation controls
                    var nav = document.createElement('div');
                    nav.className = 'diff-nav';
                    nav.innerHTML =
                        '<button type="button" class="diff-nav-btn" data-action="prev">Prev</button>' +
                        '<button type="button" class="diff-nav-btn" data-action="next">Next</button>' +
                        '<span class="diff-nav-count"><span class="diff-nav-current">1</span> of ' + totalChanges + ' changes</span>';

                    // Insert navigation before the diff-row
                    var diffRow = section.querySelector('.diff-row');
                    diffRow.parentNode.insertBefore(nav, diffRow);

                    var prevBtn = nav.querySelector('[data-action="prev"]');
                    var nextBtn = nav.querySelector('[data-action="next"]');
                    var currentSpan = nav.querySelector('.diff-nav-current');

                    // Sync scroll between panels (for manual scrolling)
                    var syncing = false;
                    function syncScroll(source, target) {
                        if (syncing) return;
                        syncing = true;
                        var scrollRatio = source.scrollTop / (source.scrollHeight - source.clientHeight || 1);
                        target.scrollTop = scrollRatio * (target.scrollHeight - target.clientHeight);
                        setTimeout(function() { syncing = false; }, 50);
                    }

                    oldEl.addEventListener('scroll', function() { syncScroll(oldEl, newEl); });
                    newEl.addEventListener('scroll', function() { syncScroll(newEl, oldEl); });

                    function scrollToChange(index) {
                        // Remove current highlight from all changes on both sides
                        oldChanges.forEach(function(el) { el.classList.remove('diff-current'); });
                        newChanges.forEach(function(el) { el.classList.remove('diff-current'); });

                        // Get corresponding elements on both sides
                        var oldChange = oldChanges[index];
                        var newChange = newChanges[index];

                        // Highlight both sides if they exist
                        if (oldChange) oldChange.classList.add('diff-current');
                        if (newChange) newChange.classList.add('diff-current');

                        // Disable sync during programmatic scroll
                        syncing = true;

                        // Scroll both panels to center on the change
                        if (newChange) {
                            var containerTop = newEl.getBoundingClientRect().top;
                            var elementTop = newChange.getBoundingClientRect().top;
                            var relativePos = elementTop - containerTop;
                            var targetScroll = newEl.scrollTop + relativePos - (newEl.clientHeight / 2);
                            newEl.scroll({ top: Math.max(0, targetScroll), behavior: 'instant' });
                        }
                        if (oldChange) {
                            var containerTop = oldEl.getBoundingClientRect().top;
                            var elementTop = oldChange.getBoundingClientRect().top;
                            var relativePos = elementTop - containerTop;
                            var targetScroll = oldEl.scrollTop + relativePos - (oldEl.clientHeight / 2);
                            oldEl.scroll({ top: Math.max(0, targetScroll), behavior: 'instant' });
                        }

                        // Re-enable sync after a delay
                        setTimeout(function() { syncing = false; }, 100);

                        // Update counter
                        currentSpan.textContent = (index + 1);
                    }

                    prevBtn.addEventListener('click', function(e) {
                        e.preventDefault();
                        currentIndex = (currentIndex - 1 + totalChanges) % totalChanges;
                        scrollToChange(currentIndex);
                    });

                    nextBtn.addEventListener('click', function(e) {
                        e.preventDefault();
                        currentIndex = (currentIndex + 1) % totalChanges;
                        scrollToChange(currentIndex);
                    });

                    // Auto-scroll to first change after a short delay
                    setTimeout(function() {
                        scrollToChange(0);
                    }, 200);
                }
            }
        });
        </script>
    ` + adminLayoutEnd,

	"content_form": adminLayoutStart + `
        <div class="page-header">
            <h1>{{if .IsNew}}New {{.Template.Name}}{{else}}Edit Content{{end}}</h1>
        </div>
        {{if .Error}}<div class="error-message">{{.Error}}</div>{{end}}
        <form method="POST" action="{{if .IsNew}}/cm/content/create{{else}}/cm/content/{{.Content.ID.Hex}}{{end}}" enctype="multipart/form-data" class="form-card">
            {{.CSRFField}}
            <input type="hidden" name="template_id" value="{{.Template.ID.Hex}}">
            <input type="hidden" name="create_redirect" id="create_redirect" value="">
            <input type="hidden" name="slug_rename_enabled" id="slug_rename_enabled" value="">
            <input type="hidden" name="version_comment" id="version_comment" value="">

            {{if not .IsNew}}
            <div class="form-group" style="background: var(--bg-tertiary); padding: 1rem; border-radius: var(--radius); margin-bottom: 1.5rem;">
                <div style="display: flex; align-items: center; justify-content: space-between;">
                    <div>
                        <label style="margin-bottom: 0.25rem; display: block;">Template</label>
                        <span style="font-size: 1.1rem; font-weight: 500;">{{.Template.Name}}</span>
                    </div>
                    <button type="button" class="btn btn-sm btn-outline" onclick="showChangeTemplateModal()">Change Template</button>
                </div>
            </div>
            {{end}}

            <div class="form-group">
                <label for="title">Title</label>
                <input type="text" id="title" name="title" value="{{if .Content}}{{.Content.Title}}{{end}}" required>
            </div>
            <div class="form-group">
                <label for="slug">Slug (URL path)</label>
                <div class="slug-input-wrapper" style="display: flex; gap: 0.5rem; align-items: center;">
                    {{if .IsNew}}
                    <input type="text" id="slug" name="slug" value="" placeholder="auto-generated from title" style="flex: 1;">
                    {{else}}
                    <input type="text" id="slug" name="slug" value="{{.Content.Slug}}" readonly style="flex: 1; background: var(--bg-tertiary);">
                    <button type="button" id="rename-slug-btn" class="btn btn-sm btn-outline" onclick="enableSlugRename()">Rename</button>
                    <button type="button" id="cancel-rename-btn" class="btn btn-sm btn-outline" onclick="cancelSlugRename()" style="display: none;">Cancel</button>
                    {{end}}
                </div>
                <p class="help-text slug-error" id="slug-error" style="color: var(--error); display: none;"></p>
                {{if not .IsNew}}<p class="help-text">Leave empty for root page (/). Click Rename to change the slug.</p>{{end}}
            </div>
            <div class="form-group">
                <label for="folder_id">Folder</label>
                <select id="folder_id" name="folder_id">
                    <option value="root">/ (root)</option>
                    {{range .Folders}}
                    <option value="{{.ID.Hex}}" {{if $.Content}}{{if $.Content.FolderID}}{{if eq .ID.Hex $.Content.FolderID.Hex}}selected{{end}}{{end}}{{end}}>{{.Path}}</option>
                    {{end}}
                </select>
                <p class="help-text">The full URL will be: <code id="preview-path">/</code></p>
            </div>

            {{range .Template.Fields}}
            <div class="form-group">
                <label for="field_{{.Name}}">{{.Label}}{{if .Required}} *{{end}}</label>
                {{if eq .Type "text"}}
                <input type="text" id="field_{{.Name}}" name="field_{{.Name}}"
                    value="{{if $.Content}}{{index $.Content.Data .Name}}{{end}}"
                    placeholder="{{.Placeholder}}" {{if .Required}}required{{end}}>
                {{else if eq .Type "textarea"}}
                <textarea id="field_{{.Name}}" name="field_{{.Name}}" rows="4"
                    placeholder="{{.Placeholder}}" {{if .Required}}required{{end}}>{{if $.Content}}{{index $.Content.Data .Name}}{{end}}</textarea>
                {{else if eq .Type "richtext"}}
                <div class="richtext-toggle" style="margin-bottom: 0.5rem;">
                    <button type="button" class="btn btn-sm btn-outline toggle-html-btn" onclick="toggleFieldHtmlMode(this, 'field_{{.Name}}')" title="Toggle between rich editor and raw HTML">
                        &lt;/&gt; Edit HTML
                    </button>
                </div>
                <textarea id="field_{{.Name}}" name="field_{{.Name}}" class="richtext"
                    {{if .Required}}required{{end}}>{{if $.Content}}{{index $.Content.Data .Name}}{{end}}</textarea>
                {{else if eq .Type "rawhtml"}}
                <textarea id="field_{{.Name}}" name="field_{{.Name}}" rows="20" class="code-editor"
                    placeholder="{{.Placeholder}}" {{if .Required}}required{{end}}>{{if $.Content}}{{index $.Content.Data .Name}}{{end}}</textarea>
                {{else if eq .Type "date"}}
                <input type="date" id="field_{{.Name}}" name="field_{{.Name}}"
                    value="{{if $.Content}}{{index $.Content.Data .Name}}{{end}}" {{if .Required}}required{{end}}>
                {{else if eq .Type "image"}}
                {{if $.Content}}{{if index $.Content.Data .Name}}
                <div class="current-image">
                    <img src="{{index $.Content.Data .Name}}" alt="Current image" style="max-width: 200px; margin-bottom: 0.5rem;">
                </div>
                {{end}}{{end}}
                <input type="file" id="field_{{.Name}}" name="field_{{.Name}}" accept="image/*">
                {{else if eq .Type "select"}}
                <select id="field_{{.Name}}" name="field_{{.Name}}" {{if .Required}}required{{end}}>
                    <option value="">Select...</option>
                    {{$currentVal := ""}}{{if $.Content}}{{$currentVal = index $.Content.Data .Name}}{{end}}
                    {{range $opt := (split .Options ",")}}
                    <option value="{{$opt}}" {{if eq $opt $currentVal}}selected{{end}}>{{$opt}}</option>
                    {{end}}
                </select>
                {{end}}
            </div>
            {{end}}

            <div class="form-section">
                <h3>SEO Settings</h3>
                <div class="form-group">
                    <label for="meta_description">Meta Description</label>
                    <textarea id="meta_description" name="meta_description" rows="2" placeholder="Brief description for search engines (150-160 characters)">{{if .Content}}{{.Content.MetaDescription}}{{end}}</textarea>
                    <p class="help-text">Recommended: 150-160 characters for best SEO results</p>
                </div>
                <div class="form-group">
                    <label for="og_image">Social Share Image (OG Image)</label>
                    {{if .Content}}{{if .Content.OGImage}}
                    <div class="current-image">
                        <img src="{{.Content.OGImage}}" alt="Current OG image" style="max-width: 200px; margin-bottom: 0.5rem;">
                    </div>
                    {{end}}{{end}}
                    <input type="file" id="og_image" name="og_image" accept="image/*">
                    <p class="help-text">Recommended: 1200x630 pixels for best display on social media</p>
                </div>
            </div>

            <div class="form-section">
                <h3>Page Settings</h3>
                <div class="form-group checkbox-group">
                    <label class="checkbox-label">
                        <input type="checkbox" name="published" {{if .Content}}{{if .Content.Published}}checked{{end}}{{end}}>
                        Published
                    </label>
                </div>
                <div class="form-group checkbox-group">
                    <label class="checkbox-label">
                        <input type="hidden" name="use_header" value="off">
                        <input type="checkbox" name="use_header" value="on" {{if .IsNew}}checked{{else}}{{if .Content.UseHeader}}checked{{end}}{{end}}>
                        Include site header
                    </label>
                </div>
                <div class="form-group checkbox-group">
                    <label class="checkbox-label">
                        <input type="hidden" name="use_footer" value="off">
                        <input type="checkbox" name="use_footer" value="on" {{if .IsNew}}checked{{else}}{{if .Content.UseFooter}}checked{{end}}{{end}}>
                        Include site footer
                    </label>
                </div>
                {{if eq .Template.Slug "blank-page"}}
                <div class="form-group checkbox-group">
                    <label class="checkbox-label">
                        <input type="hidden" name="use_theme" value="off">
                        <input type="checkbox" name="use_theme" value="on" id="use_theme" {{if .IsNew}}checked{{else}}{{if .Content.UseTheme}}checked{{end}}{{end}}>
                        Use site theme (CSS, layout wrapper)
                    </label>
                </div>
                {{end}}
            </div>

            <div class="form-actions">
                <a href="/cm/content" class="btn btn-outline">Cancel</a>
                <button type="submit" class="btn btn-primary">{{if .IsNew}}Create{{else}}Update{{end}}</button>
            </div>
        </form>

        <!-- Redirect confirmation modal -->
        <div id="redirect-modal" style="display: none; position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0, 0, 0, 0.7); z-index: 10000; align-items: center; justify-content: center;">
            <div style="background: #1e293b; border-radius: var(--radius); max-width: 500px; width: 90%; box-shadow: 0 20px 60px rgba(0, 0, 0, 0.8); border: 1px solid rgba(99, 102, 241, 0.3);">
                <div style="padding: 1.5rem; border-bottom: 1px solid rgba(99, 102, 241, 0.2); background: #1a2332;">
                    <h3 style="margin: 0; color: var(--text);">Create Redirect?</h3>
                </div>
                <div style="padding: 1.5rem; background: #1e293b;">
                    <p style="margin: 0 0 0.5rem 0;">You are changing the URL from:</p>
                    <p style="margin: 0 0 1rem 0;"><code id="redirect-old-path" style="background: #0f172a; padding: 0.25rem 0.5rem; border-radius: 4px; color: var(--accent);"></code></p>
                    <p style="margin: 0 0 0.5rem 0;">to:</p>
                    <p style="margin: 0 0 1rem 0;"><code id="redirect-new-path" style="background: #0f172a; padding: 0.25rem 0.5rem; border-radius: 4px; color: var(--accent);"></code></p>
                    <p style="margin: 0; color: var(--muted); font-size: 0.9rem;">Creating a redirect will preserve any existing links or bookmarks to the old URL.</p>
                </div>
                <div style="padding: 1rem 1.5rem; border-top: 1px solid rgba(99, 102, 241, 0.2); display: flex; gap: 0.75rem; justify-content: flex-end; background: #1a2332;">
                    <button type="button" class="btn btn-outline" id="redirect-no-btn">Rename without Redirect</button>
                    <button type="button" class="btn btn-primary" id="redirect-yes-btn">Yes, Redirect</button>
                </div>
            </div>
        </div>

        {{if not .IsNew}}
        <!-- Change template modal -->
        <div id="change-template-modal" style="display: none; position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0, 0, 0, 0.7); z-index: 10000; align-items: center; justify-content: center;">
            <div style="background: #1e293b; border-radius: var(--radius); max-width: 500px; width: 90%; box-shadow: 0 20px 60px rgba(0, 0, 0, 0.8); border: 1px solid rgba(245, 158, 11, 0.3);">
                <div style="padding: 1.5rem; border-bottom: 1px solid rgba(245, 158, 11, 0.2); background: #1a2332;">
                    <h3 style="margin: 0; color: var(--warning);">Change Template</h3>
                </div>
                <div style="padding: 1.5rem; background: #1e293b;">
                    <p style="margin: 0 0 1rem 0; color: var(--warning);">Changing templates may not preserve all field data.</p>
                    <p style="margin: 0 0 1.5rem 0; color: var(--muted);">Fields with matching names will be carried over. Fields that don't exist in the new template will be lost. You'll see a preview of the changes before confirming.</p>
                    <div class="form-group" style="margin-bottom: 0;">
                        <label for="new-template-select" style="margin-bottom: 0.5rem; display: block;">Select New Template</label>
                        <select id="new-template-select" style="width: 100%; padding: 0.75rem; background: var(--bg-tertiary); border: 1px solid var(--border); border-radius: var(--radius); color: var(--text);">
                            {{range .AllTemplates}}
                            {{if ne .ID.Hex $.Template.ID.Hex}}
                            <option value="{{.ID.Hex}}">{{.Name}}</option>
                            {{end}}
                            {{end}}
                        </select>
                    </div>
                </div>
                <div style="padding: 1rem 1.5rem; border-top: 1px solid rgba(245, 158, 11, 0.2); display: flex; gap: 0.75rem; justify-content: flex-end; background: #1a2332;">
                    <button type="button" class="btn btn-outline" onclick="closeChangeTemplateModal()">Cancel</button>
                    <button type="button" class="btn" style="background: var(--warning); color: white;" onclick="proceedToTemplatePreview()">Proceed</button>
                </div>
            </div>
        </div>

        <!-- Version comment modal -->
        <div id="version-comment-modal" style="display: none; position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0, 0, 0, 0.7); z-index: 10000; align-items: center; justify-content: center;">
            <div style="background: #1e293b; border-radius: var(--radius); max-width: 500px; width: 90%; box-shadow: 0 20px 60px rgba(0, 0, 0, 0.8); border: 1px solid rgba(99, 102, 241, 0.3);">
                <div style="padding: 1.5rem; border-bottom: 1px solid rgba(99, 102, 241, 0.2); background: #1a2332;">
                    <h3 style="margin: 0; color: var(--text);">Save Version <span id="version-number-display" style="color: var(--accent);"></span></h3>
                </div>
                <div style="padding: 1.5rem; background: #1e293b;">
                    <p style="margin: 0 0 1rem 0; color: var(--muted);">Add an optional comment to describe this version's changes.</p>
                    <div class="form-group" style="margin-bottom: 0;">
                        <label for="version-comment-input" style="margin-bottom: 0.5rem; display: block;">Version Comment (optional)</label>
                        <textarea id="version-comment-input" rows="3" style="width: 100%; padding: 0.75rem; background: var(--bg-tertiary); border: 1px solid var(--border); border-radius: var(--radius); color: var(--text); resize: vertical;" placeholder="e.g., Updated hero image, Fixed typo in introduction..."></textarea>
                    </div>
                </div>
                <div style="padding: 1rem 1.5rem; border-top: 1px solid rgba(99, 102, 241, 0.2); display: flex; gap: 0.75rem; justify-content: flex-end; background: #1a2332;">
                    <button type="button" class="btn btn-outline" id="version-comment-cancel">Cancel</button>
                    <button type="button" class="btn btn-primary" id="version-comment-save">Save</button>
                </div>
            </div>
        </div>
        {{end}}

        {{if not .IsNew}}
        {{if .Content.Deleted}}
        <div class="form-section" style="margin-top: 2rem; background: rgba(239, 68, 68, 0.1); border: 1px solid rgba(239, 68, 68, 0.3); border-radius: var(--radius); padding: 1.5rem;">
            <h3 style="color: var(--danger);">⚠️ This content is deleted</h3>
            <p style="margin-bottom: 1rem;">This page was deleted on {{if .Content.DeletedAt}}{{.Content.DeletedAt.Format "Jan 2, 2006 3:04 PM"}}{{else}}unknown date{{end}}.</p>
            <form method="POST" action="/cm/content/{{.Content.ID.Hex}}/undelete" style="display: inline;">
            {{.CSRFField}}
                <button type="submit" class="btn btn-primary">Restore This Page</button>
            </form>
        </div>
        {{end}}

        {{if .Versions}}
        <div class="form-section" style="margin-top: 2rem;">
            <h3>Version History</h3>
            <p style="color: var(--muted); margin-bottom: 1rem;">Previous versions of this page are saved automatically when you update.</p>
            <div class="table-container">
                <table>
                    <thead>
                        <tr>
                            <th>Version</th>
                            <th>Title</th>
                            <th>Comment</th>
                            <th>Saved</th>
                            <th>Actions</th>
                        </tr>
                    </thead>
                    <tbody>
                        {{range .Versions}}
                        <tr>
                            <td>v{{.Version}}</td>
                            <td>{{.Title}}</td>
                            <td style="color: var(--muted); font-size: 0.9rem; max-width: 200px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;" title="{{.Comment}}">{{if .Comment}}{{.Comment}}{{else}}-{{end}}</td>
                            <td>{{.CreatedAt.Format "Jan 2, 2006 3:04 PM"}}</td>
                            <td class="actions">
                                <a href="/cm/content/{{.ContentID.Hex}}/versions/{{.Version}}/diff" class="btn btn-sm btn-outline">Diff</a>
                                <a href="/cm/content/{{.ContentID.Hex}}/versions/{{.Version}}/view" target="_blank" class="btn btn-sm btn-outline">Preview</a>
                                <form method="POST" action="/cm/content/{{.ContentID.Hex}}/versions/{{.Version}}/revert" style="display:inline" onsubmit="return confirmRevert(this, {{.Version}})">
            {{$.CSRFField}}
                                    <button type="submit" class="btn btn-sm btn-secondary">Revert</button>
                                </form>
                            </td>
                        </tr>
                        {{end}}
                    </tbody>
                </table>
            </div>
        </div>
        {{end}}

        {{if .SameSlugPages}}
        <div class="form-section" style="margin-top: 2rem;">
            <h3>Historical Pages at This URL</h3>
            <p style="color: var(--muted); margin-bottom: 1rem;">These are other pages that have used the same slug "{{.Content.Slug}}" (different document IDs).</p>
            <div class="table-container">
                <table>
                    <thead>
                        <tr>
                            <th>Title</th>
                            <th>Status</th>
                            <th>Last Updated</th>
                            <th>Actions</th>
                        </tr>
                    </thead>
                    <tbody>
                        {{range .SameSlugPages}}
                        <tr{{if .Deleted}} style="opacity: 0.7;"{{end}}>
                            <td>{{.Title}}</td>
                            <td>
                                {{if .Deleted}}
                                <span class="status-badge" style="background: var(--danger);">Deleted</span>
                                {{else if .Published}}
                                <span class="status-badge published">Published</span>
                                {{else}}
                                <span class="status-badge draft">Draft</span>
                                {{end}}
                            </td>
                            <td>{{.UpdatedAt.Format "Jan 2, 2006 3:04 PM"}}</td>
                            <td class="actions">
                                <a href="/cm/content/{{.ID.Hex}}" class="btn btn-sm">Edit</a>
                            </td>
                        </tr>
                        {{end}}
                    </tbody>
                </table>
            </div>
        </div>
        {{end}}
        {{end}}

        <link href="https://cdn.jsdelivr.net/npm/quill@2.0.3/dist/quill.snow.css" rel="stylesheet">
        <script src="https://cdn.jsdelivr.net/npm/quill@2.0.3/dist/quill.js"></script>
        <style>
            .quill-wrapper { margin-bottom: 1rem; }
            .quill-wrapper .ql-toolbar { background: rgba(15, 23, 42, 0.5); border-color: var(--border); border-radius: var(--radius) var(--radius) 0 0; }
            .quill-wrapper .ql-container { background: rgba(15, 23, 42, 0.5); border-color: var(--border); border-radius: 0 0 var(--radius) var(--radius); min-height: 300px; }
            .quill-wrapper .ql-editor { color: var(--text); min-height: 280px; font-size: 1rem; }
            .quill-wrapper .ql-editor.ql-blank::before { color: var(--text-muted); }
            .ql-toolbar .ql-stroke { stroke: var(--text); }
            .ql-toolbar .ql-fill { fill: var(--text); }
            .ql-toolbar .ql-picker { color: var(--text); }
            .ql-toolbar .ql-picker-options { background: var(--bg-card); border-color: var(--border); }
            .ql-toolbar button:hover, .ql-toolbar button.ql-active { color: var(--primary); }
            .ql-toolbar button:hover .ql-stroke, .ql-toolbar button.ql-active .ql-stroke { stroke: var(--primary); }

            /* Search and Replace styles */
            .search-replace-panel {
                position: fixed;
                top: 50%;
                left: 50%;
                transform: translate(-50%, -50%);
                background: var(--bg-card);
                border: 1px solid var(--border);
                border-radius: var(--radius);
                padding: 1.5rem;
                width: 100%;
                max-width: 500px;
                box-shadow: 0 20px 50px rgba(0, 0, 0, 0.5);
                z-index: 10001;
            }
            .search-replace-overlay {
                position: fixed;
                top: 0;
                left: 0;
                right: 0;
                bottom: 0;
                background: rgba(0, 0, 0, 0.7);
                z-index: 10000;
            }
            .search-replace-panel h3 {
                margin-bottom: 1rem;
                font-size: 1.25rem;
                color: var(--text);
                display: flex;
                align-items: center;
                gap: 0.5rem;
            }
            .search-replace-row {
                display: flex;
                gap: 0.5rem;
                margin-bottom: 0.75rem;
                align-items: center;
            }
            .search-replace-row label {
                width: 70px;
                color: var(--text);
                font-weight: 500;
                font-size: 0.9rem;
            }
            .search-replace-row input {
                flex: 1;
                padding: 0.6rem 0.75rem;
                background: rgba(15, 23, 42, 0.5);
                border: 1px solid var(--border);
                border-radius: var(--radius);
                color: var(--text);
                font-size: 0.95rem;
            }
            .search-replace-row input:focus {
                outline: none;
                border-color: var(--primary);
                box-shadow: 0 0 0 3px rgba(99, 102, 241, 0.2);
            }
            .search-replace-options {
                display: flex;
                gap: 1rem;
                margin-bottom: 1rem;
                padding: 0.5rem 0;
            }
            .search-replace-options label {
                display: flex;
                align-items: center;
                gap: 0.5rem;
                color: var(--text-muted);
                font-size: 0.85rem;
                cursor: pointer;
            }
            .search-replace-options input[type="checkbox"] {
                width: 16px;
                height: 16px;
                cursor: pointer;
            }
            .search-replace-status {
                padding: 0.5rem 0.75rem;
                background: rgba(15, 23, 42, 0.3);
                border-radius: var(--radius);
                margin-bottom: 1rem;
                font-size: 0.85rem;
                color: var(--text-muted);
                min-height: 36px;
                display: flex;
                align-items: center;
            }
            .search-replace-status.has-matches {
                color: var(--accent);
            }
            .search-replace-status.no-matches {
                color: var(--danger);
            }
            .search-replace-actions {
                display: flex;
                gap: 0.5rem;
                flex-wrap: wrap;
                padding-top: 1rem;
                border-top: 1px solid var(--border);
            }
            .search-replace-actions .btn {
                padding: 0.5rem 1rem;
                font-size: 0.9rem;
            }
            .search-replace-actions .btn-group {
                display: flex;
                gap: 0.5rem;
            }
            .search-replace-actions .spacer {
                flex: 1;
            }
            .search-highlight {
                background-color: rgba(250, 204, 21, 0.4) !important;
                color: inherit !important;
            }
            .search-highlight-current {
                background-color: rgba(250, 204, 21, 0.8) !important;
                color: #000 !important;
                outline: 2px solid var(--primary);
            }
            .ql-search-replace {
                width: auto !important;
                padding: 0 8px !important;
                font-size: 12px !important;
            }
            .ql-search-replace::after {
                content: 'Find';
            }
            /* Custom link modal styles */
            .link-modal-overlay {
                position: fixed;
                top: 0;
                left: 0;
                right: 0;
                bottom: 0;
                background: rgba(0, 0, 0, 0.7);
                display: flex;
                align-items: center;
                justify-content: center;
                z-index: 10000;
            }
            .link-modal {
                background: var(--bg-card);
                border: 1px solid var(--border);
                border-radius: var(--radius);
                padding: 1.5rem;
                width: 100%;
                max-width: 500px;
                box-shadow: 0 20px 50px rgba(0, 0, 0, 0.5);
            }
            .link-modal h3 {
                margin-bottom: 1.5rem;
                font-size: 1.25rem;
                color: var(--text);
            }
            .link-type-tabs {
                display: flex;
                gap: 0.5rem;
                margin-bottom: 1.5rem;
            }
            .link-type-tab {
                flex: 1;
                padding: 0.75rem;
                border: 1px solid var(--border);
                border-radius: var(--radius);
                background: transparent;
                color: var(--text-muted);
                cursor: pointer;
                font-size: 0.9rem;
                transition: all 0.2s;
            }
            .link-type-tab:hover {
                border-color: var(--primary);
                color: var(--text);
            }
            .link-type-tab.active {
                background: linear-gradient(135deg, var(--primary), var(--secondary));
                border-color: transparent;
                color: white;
            }
            .link-input-group {
                margin-bottom: 1rem;
                position: relative;
            }
            .link-input-group label {
                display: block;
                margin-bottom: 0.5rem;
                color: var(--text);
                font-weight: 500;
            }
            .link-input-group input {
                width: 100%;
                padding: 0.75rem 1rem;
                background: rgba(15, 23, 42, 0.5);
                border: 1px solid var(--border);
                border-radius: var(--radius);
                color: var(--text);
                font-size: 1rem;
            }
            .link-input-group input:focus {
                outline: none;
                border-color: var(--primary);
                box-shadow: 0 0 0 3px rgba(99, 102, 241, 0.2);
            }
            .autocomplete-dropdown {
                position: absolute;
                top: 100%;
                left: 0;
                right: 0;
                background: var(--bg-card);
                border: 1px solid var(--border);
                border-top: none;
                border-radius: 0 0 var(--radius) var(--radius);
                max-height: 200px;
                overflow-y: auto;
                z-index: 10001;
                display: none;
            }
            .autocomplete-dropdown.show {
                display: block;
            }
            .autocomplete-item {
                padding: 0.75rem 1rem;
                cursor: pointer;
                border-bottom: 1px solid var(--border);
            }
            .autocomplete-item:last-child {
                border-bottom: none;
            }
            .autocomplete-item:hover,
            .autocomplete-item.selected {
                background: var(--bg-hover);
            }
            .autocomplete-item .slug {
                font-family: 'JetBrains Mono', monospace;
                font-size: 0.85rem;
                color: var(--accent);
            }
            .autocomplete-item .title {
                font-size: 0.8rem;
                color: var(--text-muted);
                margin-top: 0.25rem;
            }
            .link-modal-actions {
                display: flex;
                gap: 1rem;
                justify-content: flex-end;
                margin-top: 1.5rem;
                padding-top: 1rem;
                border-top: 1px solid var(--border);
            }
            .link-hint {
                font-size: 0.8rem;
                color: var(--text-muted);
                margin-top: 0.5rem;
            }
        </style>
        <script>
        // Global slugs cache for autocomplete
        var siteSlugs = [];

        // Load all slugs for autocomplete
        function loadSlugs() {
            fetch('/api/slugs')
                .then(function(res) { return res.json(); })
                .then(function(data) { siteSlugs = data; })
                .catch(function(err) { console.log('Failed to load slugs:', err); });
        }

        // Custom link handler
        function createLinkModal(quill, existingLink) {
            var selection = quill.getSelection();
            var selectedText = selection ? quill.getText(selection.index, selection.length) : '';

            var overlay = document.createElement('div');
            overlay.className = 'link-modal-overlay';

            var modal = document.createElement('div');
            modal.className = 'link-modal';
            modal.innerHTML = ` + "`" + `
                <h3>${existingLink ? 'Edit Link' : 'Insert Link'}</h3>
                <div class="link-type-tabs">
                    <button type="button" class="link-type-tab active" data-type="internal">Internal Page</button>
                    <button type="button" class="link-type-tab" data-type="external">External URL</button>
                </div>
                <div id="internal-link-section">
                    <div class="link-input-group">
                        <label>Select Page</label>
                        <input type="text" id="internal-link-input" placeholder="Start typing to search pages..." autocomplete="off">
                        <div class="autocomplete-dropdown" id="autocomplete-dropdown"></div>
                        <p class="link-hint">Type to search, use Tab to accept suggestion</p>
                    </div>
                </div>
                <div id="external-link-section" style="display: none;">
                    <div class="link-input-group">
                        <label>URL</label>
                        <input type="text" id="external-link-input" placeholder="https://example.com">
                    </div>
                </div>
                <div class="link-modal-actions">
                    <button type="button" class="btn btn-outline" id="link-cancel">Cancel</button>
                    ${existingLink ? '<button type="button" class="btn btn-danger" id="link-remove">Remove Link</button>' : ''}
                    <button type="button" class="btn btn-primary" id="link-save">Save</button>
                </div>
            ` + "`" + `;

            overlay.appendChild(modal);
            document.body.appendChild(overlay);

            var internalInput = modal.querySelector('#internal-link-input');
            var externalInput = modal.querySelector('#external-link-input');
            var dropdown = modal.querySelector('#autocomplete-dropdown');
            var currentLinkType = 'internal';
            var selectedIndex = -1;
            var filteredSlugs = [];

            // Pre-fill with existing link
            if (existingLink) {
                if (existingLink.startsWith('/')) {
                    internalInput.value = existingLink;
                    currentLinkType = 'internal';
                } else {
                    externalInput.value = existingLink;
                    currentLinkType = 'external';
                    modal.querySelector('[data-type="internal"]').classList.remove('active');
                    modal.querySelector('[data-type="external"]').classList.add('active');
                    modal.querySelector('#internal-link-section').style.display = 'none';
                    modal.querySelector('#external-link-section').style.display = 'block';
                }
            }

            // Tab switching
            modal.querySelectorAll('.link-type-tab').forEach(function(tab) {
                tab.addEventListener('click', function() {
                    modal.querySelectorAll('.link-type-tab').forEach(function(t) { t.classList.remove('active'); });
                    tab.classList.add('active');
                    currentLinkType = tab.dataset.type;
                    if (currentLinkType === 'internal') {
                        modal.querySelector('#internal-link-section').style.display = 'block';
                        modal.querySelector('#external-link-section').style.display = 'none';
                        internalInput.focus();
                    } else {
                        modal.querySelector('#internal-link-section').style.display = 'none';
                        modal.querySelector('#external-link-section').style.display = 'block';
                        externalInput.focus();
                    }
                });
            });

            // Autocomplete functionality
            function updateAutocomplete() {
                var query = internalInput.value.toLowerCase();
                filteredSlugs = siteSlugs.filter(function(item) {
                    return item.slug.toLowerCase().includes(query) || item.title.toLowerCase().includes(query);
                }).slice(0, 10);

                if (filteredSlugs.length > 0 && query.length > 0) {
                    dropdown.innerHTML = filteredSlugs.map(function(item, i) {
                        return '<div class="autocomplete-item' + (i === selectedIndex ? ' selected' : '') + '" data-slug="' + item.slug + '">' +
                            '<div class="slug">' + item.slug + '</div>' +
                            '<div class="title">' + item.title + '</div>' +
                        '</div>';
                    }).join('');
                    dropdown.classList.add('show');
                } else {
                    dropdown.classList.remove('show');
                }
            }

            internalInput.addEventListener('input', function() {
                selectedIndex = -1;
                updateAutocomplete();
            });

            internalInput.addEventListener('keydown', function(e) {
                if (e.key === 'Tab' && filteredSlugs.length > 0) {
                    e.preventDefault();
                    var idx = selectedIndex >= 0 ? selectedIndex : 0;
                    internalInput.value = filteredSlugs[idx].slug;
                    dropdown.classList.remove('show');
                } else if (e.key === 'ArrowDown') {
                    e.preventDefault();
                    selectedIndex = Math.min(selectedIndex + 1, filteredSlugs.length - 1);
                    updateAutocomplete();
                } else if (e.key === 'ArrowUp') {
                    e.preventDefault();
                    selectedIndex = Math.max(selectedIndex - 1, -1);
                    updateAutocomplete();
                } else if (e.key === 'Enter') {
                    e.preventDefault();
                    if (selectedIndex >= 0 && filteredSlugs[selectedIndex]) {
                        internalInput.value = filteredSlugs[selectedIndex].slug;
                        dropdown.classList.remove('show');
                    } else {
                        modal.querySelector('#link-save').click();
                    }
                } else if (e.key === 'Escape') {
                    overlay.remove();
                }
            });

            dropdown.addEventListener('click', function(e) {
                var item = e.target.closest('.autocomplete-item');
                if (item) {
                    internalInput.value = item.dataset.slug;
                    dropdown.classList.remove('show');
                }
            });

            // Close dropdown when clicking outside
            document.addEventListener('click', function closeDropdown(e) {
                if (!dropdown.contains(e.target) && e.target !== internalInput) {
                    dropdown.classList.remove('show');
                }
            });

            // Save link
            modal.querySelector('#link-save').addEventListener('click', function() {
                var link = currentLinkType === 'internal' ? internalInput.value : externalInput.value;
                if (link) {
                    // Ensure internal links start with /
                    if (currentLinkType === 'internal' && !link.startsWith('/')) {
                        link = '/' + link;
                    }
                    if (selection && selection.length > 0) {
                        quill.formatText(selection.index, selection.length, 'link', link);
                    } else {
                        // Insert link at cursor position
                        var text = selectedText || link;
                        quill.insertText(selection ? selection.index : 0, text, 'link', link);
                    }
                }
                overlay.remove();
            });

            // Cancel
            modal.querySelector('#link-cancel').addEventListener('click', function() {
                overlay.remove();
            });

            // Remove link
            var removeBtn = modal.querySelector('#link-remove');
            if (removeBtn) {
                removeBtn.addEventListener('click', function() {
                    if (selection && selection.length > 0) {
                        quill.formatText(selection.index, selection.length, 'link', false);
                    }
                    overlay.remove();
                });
            }

            // Close on overlay click
            overlay.addEventListener('click', function(e) {
                if (e.target === overlay) {
                    overlay.remove();
                }
            });

            // Focus appropriate input
            setTimeout(function() {
                if (currentLinkType === 'internal') {
                    internalInput.focus();
                } else {
                    externalInput.focus();
                }
            }, 100);
        }

        // Search and Replace functionality
        var searchReplaceState = {
            quill: null,
            matches: [],
            currentIndex: -1,
            searchTerm: '',
            caseSensitive: false,
            overlay: null
        };

        function openSearchReplace(quill) {
            searchReplaceState.quill = quill;
            searchReplaceState.matches = [];
            searchReplaceState.currentIndex = -1;

            var overlay = document.createElement('div');
            overlay.className = 'search-replace-overlay';
            searchReplaceState.overlay = overlay;

            var panel = document.createElement('div');
            panel.className = 'search-replace-panel';
            panel.innerHTML = ` + "`" + `
                <h3>
                    <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor" width="20" height="20">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                    </svg>
                    Find and Replace
                </h3>
                <div class="search-replace-row">
                    <label>Find:</label>
                    <input type="text" id="sr-search" placeholder="Search text..." autofocus>
                </div>
                <div class="search-replace-row">
                    <label>Replace:</label>
                    <input type="text" id="sr-replace" placeholder="Replacement text...">
                </div>
                <div class="search-replace-options">
                    <label>
                        <input type="checkbox" id="sr-case-sensitive">
                        Case sensitive
                    </label>
                </div>
                <div class="search-replace-status" id="sr-status">
                    Enter search text to find matches
                </div>
                <div class="search-replace-actions">
                    <div class="btn-group">
                        <button type="button" class="btn btn-outline" id="sr-prev">Previous</button>
                        <button type="button" class="btn btn-outline" id="sr-next">Next</button>
                    </div>
                    <div class="btn-group">
                        <button type="button" class="btn btn-outline" id="sr-replace-one">Replace</button>
                        <button type="button" class="btn btn-primary" id="sr-replace-all">Replace All</button>
                    </div>
                    <div class="spacer"></div>
                    <button type="button" class="btn btn-outline" id="sr-close">Close</button>
                </div>
            ` + "`" + `;

            overlay.appendChild(panel);
            document.body.appendChild(overlay);

            var searchInput = document.getElementById('sr-search');
            var caseSensitive = document.getElementById('sr-case-sensitive');

            searchInput.addEventListener('input', function() { performSearch(); });
            caseSensitive.addEventListener('change', function() {
                searchReplaceState.caseSensitive = this.checked;
                performSearch();
            });
            document.getElementById('sr-prev').addEventListener('click', function() { navigateMatch(-1); });
            document.getElementById('sr-next').addEventListener('click', function() { navigateMatch(1); });
            document.getElementById('sr-replace-one').addEventListener('click', function() { replaceCurrentMatch(); });
            document.getElementById('sr-replace-all').addEventListener('click', function() { replaceAllWithReview(); });
            document.getElementById('sr-close').addEventListener('click', function() { closeSearchReplace(); });

            overlay.addEventListener('keydown', function(e) {
                if (e.key === 'Escape') closeSearchReplace();
                else if (e.key === 'Enter' && e.target.id === 'sr-search') { e.preventDefault(); navigateMatch(1); }
                else if (e.key === 'Enter' && e.target.id === 'sr-replace') { e.preventDefault(); replaceCurrentMatch(); }
            });
            overlay.addEventListener('click', function(e) { if (e.target === overlay) closeSearchReplace(); });
            searchInput.focus();
        }

        function performSearch() {
            var searchInput = document.getElementById('sr-search');
            var status = document.getElementById('sr-status');
            var searchTerm = searchInput.value;
            searchReplaceState.searchTerm = searchTerm;
            clearHighlights();
            if (!searchTerm) {
                searchReplaceState.matches = [];
                searchReplaceState.currentIndex = -1;
                status.textContent = 'Enter search text to find matches';
                status.className = 'search-replace-status';
                return;
            }
            var quill = searchReplaceState.quill;
            var text = quill.getText();
            var matches = [];
            var searchText = searchReplaceState.caseSensitive ? searchTerm : searchTerm.toLowerCase();
            var contentText = searchReplaceState.caseSensitive ? text : text.toLowerCase();
            var pos = 0;
            while ((pos = contentText.indexOf(searchText, pos)) !== -1) {
                matches.push({ index: pos, length: searchTerm.length });
                pos += 1;
            }
            searchReplaceState.matches = matches;
            if (matches.length > 0) {
                searchReplaceState.currentIndex = 0;
                highlightMatches();
                status.textContent = 'Found ' + matches.length + ' match' + (matches.length > 1 ? 'es' : '') + ' - showing 1 of ' + matches.length;
                status.className = 'search-replace-status has-matches';
            } else {
                searchReplaceState.currentIndex = -1;
                status.textContent = 'No matches found';
                status.className = 'search-replace-status no-matches';
            }
        }

        function highlightMatches() {
            var quill = searchReplaceState.quill;
            var matches = searchReplaceState.matches;
            var currentIndex = searchReplaceState.currentIndex;
            matches.forEach(function(match, idx) {
                quill.formatText(match.index, match.length, 'background', idx === currentIndex ? '#facc15' : 'rgba(250, 204, 21, 0.4)');
            });
            if (currentIndex >= 0 && matches[currentIndex]) {
                var bounds = quill.getBounds(matches[currentIndex].index, matches[currentIndex].length);
                quill.root.scrollTop = bounds.top - quill.root.clientHeight / 2;
            }
        }

        function clearHighlights() {
            var quill = searchReplaceState.quill;
            if (quill) quill.formatText(0, quill.getLength(), 'background', false);
        }

        function navigateMatch(direction) {
            var matches = searchReplaceState.matches;
            if (matches.length === 0) return;
            var newIndex = searchReplaceState.currentIndex + direction;
            if (newIndex < 0) newIndex = matches.length - 1;
            if (newIndex >= matches.length) newIndex = 0;
            searchReplaceState.currentIndex = newIndex;
            clearHighlights();
            highlightMatches();
            var status = document.getElementById('sr-status');
            status.textContent = 'Found ' + matches.length + ' match' + (matches.length > 1 ? 'es' : '') + ' - showing ' + (newIndex + 1) + ' of ' + matches.length;
        }

        function replaceCurrentMatch() {
            var matches = searchReplaceState.matches;
            var currentIndex = searchReplaceState.currentIndex;
            if (currentIndex < 0 || !matches[currentIndex]) return;
            var quill = searchReplaceState.quill;
            var replaceText = document.getElementById('sr-replace').value;
            var match = matches[currentIndex];
            quill.formatText(match.index, match.length, 'background', false);
            quill.deleteText(match.index, match.length);
            quill.insertText(match.index, replaceText);
            performSearch();
        }

        function replaceAllWithReview() {
            var matches = searchReplaceState.matches;
            if (matches.length === 0) return;
            var quill = searchReplaceState.quill;
            var replaceText = document.getElementById('sr-replace').value;
            var searchTerm = searchReplaceState.searchTerm;
            var status = document.getElementById('sr-status');
            var reviewIndex = 0, replacedCount = 0, skippedCount = 0;

            function escapeHtml(text) {
                var div = document.createElement('div');
                div.textContent = text;
                return div.innerHTML;
            }

            function restoreActions() {
                var actionsDiv = document.querySelector('.search-replace-actions');
                actionsDiv.innerHTML = '<div class="btn-group"><button type="button" class="btn btn-outline" id="sr-prev">Previous</button><button type="button" class="btn btn-outline" id="sr-next">Next</button></div><div class="btn-group"><button type="button" class="btn btn-outline" id="sr-replace-one">Replace</button><button type="button" class="btn btn-primary" id="sr-replace-all">Replace All</button></div><div class="spacer"></div><button type="button" class="btn btn-outline" id="sr-close">Close</button>';
                document.getElementById('sr-prev').addEventListener('click', function() { navigateMatch(-1); });
                document.getElementById('sr-next').addEventListener('click', function() { navigateMatch(1); });
                document.getElementById('sr-replace-one').addEventListener('click', function() { replaceCurrentMatch(); });
                document.getElementById('sr-replace-all').addEventListener('click', function() { replaceAllWithReview(); });
                document.getElementById('sr-close').addEventListener('click', function() { closeSearchReplace(); });
            }

            function reviewNext() {
                var text = quill.getText();
                var searchText = searchReplaceState.caseSensitive ? searchTerm : searchTerm.toLowerCase();
                var contentText = searchReplaceState.caseSensitive ? text : text.toLowerCase();
                var pos = contentText.indexOf(searchText);
                if (pos === -1) {
                    status.textContent = 'Replaced ' + replacedCount + ' matches. Done.';
                    status.className = 'search-replace-status has-matches';
                    restoreActions();
                    return;
                }
                clearHighlights();
                quill.formatText(pos, searchTerm.length, 'background', '#facc15');
                var bounds = quill.getBounds(pos, searchTerm.length);
                quill.root.scrollTop = bounds.top - quill.root.clientHeight / 2;
                status.innerHTML = 'Match ' + (reviewIndex + 1) + ': Replace "<strong>' + escapeHtml(searchTerm) + '</strong>" with "<strong>' + escapeHtml(replaceText) + '</strong>"?';

                var actionsDiv = document.querySelector('.search-replace-actions');
                actionsDiv.innerHTML = '<button type="button" class="btn btn-primary" id="sr-confirm-yes">Yes</button><button type="button" class="btn btn-outline" id="sr-confirm-skip">Skip</button><button type="button" class="btn btn-outline" id="sr-confirm-all">All Remaining</button><div class="spacer"></div><button type="button" class="btn btn-outline" id="sr-confirm-cancel">Cancel</button>';

                document.getElementById('sr-confirm-yes').addEventListener('click', function() {
                    quill.formatText(pos, searchTerm.length, 'background', false);
                    quill.deleteText(pos, searchTerm.length);
                    quill.insertText(pos, replaceText);
                    replacedCount++;
                    reviewIndex++;
                    reviewNext();
                });
                document.getElementById('sr-confirm-skip').addEventListener('click', function() {
                    quill.formatText(pos, searchTerm.length, 'background', false);
                    skippedCount++;
                    reviewIndex++;
                    // Skip this match by inserting a marker then continuing
                    reviewNext();
                });
                document.getElementById('sr-confirm-all').addEventListener('click', function() {
                    while (true) {
                        var txt = quill.getText();
                        var sTxt = searchReplaceState.caseSensitive ? searchTerm : searchTerm.toLowerCase();
                        var cTxt = searchReplaceState.caseSensitive ? txt : txt.toLowerCase();
                        var p = cTxt.indexOf(sTxt);
                        if (p === -1) break;
                        quill.deleteText(p, searchTerm.length);
                        quill.insertText(p, replaceText);
                        replacedCount++;
                    }
                    status.textContent = 'Replaced ' + replacedCount + ' matches total';
                    status.className = 'search-replace-status has-matches';
                    restoreActions();
                    performSearch();
                });
                document.getElementById('sr-confirm-cancel').addEventListener('click', function() {
                    clearHighlights();
                    status.textContent = 'Cancelled. Replaced ' + replacedCount + ' matches.';
                    restoreActions();
                    performSearch();
                });
            }
            reviewNext();
        }

        function closeSearchReplace() {
            clearHighlights();
            if (searchReplaceState.overlay) {
                searchReplaceState.overlay.remove();
                searchReplaceState.overlay = null;
            }
            searchReplaceState.quill = null;
            searchReplaceState.matches = [];
            searchReplaceState.currentIndex = -1;
        }

        // Initialize Quill with custom link handler and search/replace
        function initQuillEditor(textarea) {
            var wrapper = document.createElement('div');
            wrapper.className = 'quill-wrapper';
            var editorDiv = document.createElement('div');
            wrapper.appendChild(editorDiv);
            textarea.parentNode.insertBefore(wrapper, textarea);
            textarea.style.display = 'none';

            var quill = new Quill(editorDiv, {
                theme: 'snow',
                modules: {
                    toolbar: {
                        container: [
                            [{ 'header': [1, 2, 3, false] }],
                            ['bold', 'italic', 'underline', 'strike'],
                            [{ 'color': [] }, { 'background': [] }],
                            [{ 'list': 'ordered'}, { 'list': 'bullet' }],
                            [{ 'indent': '-1'}, { 'indent': '+1' }],
                            ['link', 'image', 'video'],
                            ['blockquote', 'code-block'],
                            ['clean'],
                            ['search-replace']
                        ],
                        handlers: {
                            'link': function(value) {
                                var selection = quill.getSelection();
                                var existingLink = null;
                                if (selection && selection.length > 0) {
                                    var format = quill.getFormat(selection);
                                    existingLink = format.link || null;
                                }
                                createLinkModal(quill, existingLink);
                            },
                            'search-replace': function() {
                                openSearchReplace(quill);
                            }
                        }
                    }
                }
            });

            // Store original HTML before Quill modifies it
            textarea.dataset.originalHtml = textarea.value;

            quill.root.innerHTML = textarea.value;

            quill.on('text-change', function() {
                textarea.value = quill.root.innerHTML;
                // Mark that content has been edited via Quill
                textarea.dataset.editedViaQuill = 'true';
            });

            textarea.form.addEventListener('submit', function() {
                textarea.value = quill.root.innerHTML;
            });

            // Store quill instance on wrapper for later access
            wrapper.quillInstance = quill;

            return quill;
        }

        document.addEventListener('DOMContentLoaded', function() {
            // Load slugs for autocomplete
            loadSlugs();

            document.querySelectorAll('.richtext').forEach(function(textarea) {
                initQuillEditor(textarea);
            });

            // Initialize raw mode if checkbox is checked
            if (document.getElementById('raw_mode') && document.getElementById('raw_mode').checked) {
                toggleEditorMode();
            }
        });

        function toggleEditorMode() {
            var rawMode = document.getElementById('raw_mode');
            if (!rawMode) return;

            var textareas = document.querySelectorAll('.richtext');
            textareas.forEach(function(textarea) {
                var wrapper = textarea.previousElementSibling;
                if (rawMode.checked) {
                    // Switch to raw mode - hide Quill, show textarea
                    if (wrapper && wrapper.classList.contains('quill-wrapper')) {
                        // Sync Quill content to textarea before hiding
                        if (wrapper.quillInstance) {
                            textarea.value = wrapper.quillInstance.root.innerHTML;
                        }
                        wrapper.style.display = 'none';
                    }
                    textarea.style.display = 'block';
                    textarea.classList.add('code-editor');
                    textarea.rows = 20;
                } else {
                    // Switch to rich mode - show Quill, hide textarea
                    if (wrapper && wrapper.classList.contains('quill-wrapper')) {
                        wrapper.style.display = 'block';
                        // Sync textarea content to Quill
                        if (wrapper.quillInstance) {
                            wrapper.quillInstance.root.innerHTML = textarea.value;
                        }
                    }
                    textarea.style.display = 'none';
                    textarea.classList.remove('code-editor');
                }
            });
        }

        function toggleFieldHtmlMode(button, fieldId) {
            var textarea = document.getElementById(fieldId);
            if (!textarea) return;

            // Find the quill-wrapper - it's inserted before the textarea by initQuillEditor
            var wrapper = textarea.previousElementSibling;
            while (wrapper && !wrapper.classList.contains('quill-wrapper')) {
                wrapper = wrapper.previousElementSibling;
            }

            var isRawMode = textarea.style.display !== 'none';

            if (!isRawMode) {
                // Switch to raw HTML mode - hide Quill, show textarea
                if (wrapper && wrapper.classList.contains('quill-wrapper')) {
                    wrapper.style.display = 'none';
                }
                // Use original HTML if user hasn't edited via Quill, otherwise use current Quill content
                if (textarea.dataset.originalHtml && textarea.dataset.editedViaQuill !== 'true') {
                    textarea.value = textarea.dataset.originalHtml;
                } else if (wrapper && wrapper.quillInstance) {
                    textarea.value = wrapper.quillInstance.root.innerHTML;
                }
                textarea.style.display = 'block';
                textarea.classList.add('code-editor');
                textarea.rows = 30;
                button.innerHTML = '&#x270D; Rich Editor';
                button.classList.add('active');
            } else {
                // Switch to rich editor mode - show Quill, hide textarea
                if (wrapper && wrapper.classList.contains('quill-wrapper')) {
                    wrapper.style.display = 'block';
                    // Sync textarea content to Quill
                    if (wrapper.quillInstance) {
                        wrapper.quillInstance.root.innerHTML = textarea.value;
                    }
                }
                // Clear the edited flag since we're syncing back to Quill
                textarea.dataset.editedViaQuill = 'true';
                textarea.style.display = 'none';
                textarea.classList.remove('code-editor');
                button.innerHTML = '&lt;/&gt; Edit HTML';
                button.classList.remove('active');
            }
        }

        // Track original slug for comparison
        var originalSlug = document.getElementById('slug') ? document.getElementById('slug').value : '';
        var slugEl = document.getElementById('slug');
        var isNewContent = slugEl && !slugEl.readOnly;
        var slugRenameEnabled = false;

        // Update URL path preview when folder or slug changes
        function updatePathPreview() {
            var folderSelect = document.getElementById('folder_id');
            var slugInput = document.getElementById('slug');
            var titleInput = document.getElementById('title');
            var preview = document.getElementById('preview-path');
            if (!preview) return;

            var folder = folderSelect ? folderSelect.options[folderSelect.selectedIndex] : null;
            var folderPath = (folder && folder.value !== 'root') ? folder.text.split(' ')[0] : '';
            var slug = slugInput ? slugInput.value : '';

            // Only auto-generate slug from title for NEW content when slug is empty
            if (isNewContent && !slug && titleInput && titleInput.value) {
                slug = titleInput.value.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');
            }

            var fullPath = folderPath ? folderPath + '/' + slug : '/' + slug;
            if (!slug) fullPath = folderPath || '/';

            preview.textContent = fullPath;
        }

        // Enable slug renaming for existing content
        function enableSlugRename() {
            var slugInput = document.getElementById('slug');
            var renameBtn = document.getElementById('rename-slug-btn');
            var cancelBtn = document.getElementById('cancel-rename-btn');
            var renameEnabledField = document.getElementById('slug_rename_enabled');

            if (slugInput && renameBtn && cancelBtn) {
                slugInput.readOnly = false;
                slugInput.style.background = '';
                renameBtn.style.display = 'none';
                cancelBtn.style.display = '';
                slugRenameEnabled = true;
                if (renameEnabledField) renameEnabledField.value = 'yes';
                slugInput.focus();
            }
        }

        // Cancel slug rename and restore original value
        function cancelSlugRename() {
            var slugInput = document.getElementById('slug');
            var renameBtn = document.getElementById('rename-slug-btn');
            var cancelBtn = document.getElementById('cancel-rename-btn');
            var errorEl = document.getElementById('slug-error');
            var renameEnabledField = document.getElementById('slug_rename_enabled');

            if (slugInput && renameBtn && cancelBtn) {
                slugInput.value = originalSlug;
                slugInput.readOnly = true;
                slugInput.style.background = 'var(--bg-tertiary)';
                renameBtn.style.display = '';
                cancelBtn.style.display = 'none';
                slugRenameEnabled = false;
                if (renameEnabledField) renameEnabledField.value = '';
                if (errorEl) errorEl.style.display = 'none';
                updatePathPreview();
            }
        }

        // Check for duplicate slug before form submission
        async function validateSlug() {
            var slugInput = document.getElementById('slug');
            var errorEl = document.getElementById('slug-error');
            var folderSelect = document.getElementById('folder_id');

            if (!slugInput || !errorEl) return true;

            var slug = slugInput.value.trim();
            var folderPath = '';
            if (folderSelect && folderSelect.value !== 'root') {
                var selectedOption = folderSelect.options[folderSelect.selectedIndex];
                folderPath = selectedOption.text.split(' ')[0];
            }

            var fullPath = folderPath ? folderPath + '/' + slug : '/' + slug;
            if (!slug) fullPath = folderPath || '/';

            // Skip validation if slug hasn't changed
            var currentFullPath = '{{if .Content}}{{.Content.FullPath}}{{end}}';
            if (!currentFullPath) currentFullPath = originalSlug ? '/' + originalSlug : '/';
            if (fullPath === currentFullPath) return true;

            try {
                var response = await fetch('/api/content/check-slug?path=' + encodeURIComponent(fullPath));
                var data = await response.json();

                if (data.exists) {
                    errorEl.textContent = 'A page already exists at "' + fullPath + '". Please choose a different slug.';
                    errorEl.style.display = 'block';
                    slugInput.focus();
                    return false;
                }
                errorEl.style.display = 'none';
                return true;
            } catch (e) {
                console.error('Error checking slug:', e);
                return true; // Allow submission, server will validate
            }
        }

        // Set up event listeners for path preview
        document.addEventListener('DOMContentLoaded', function() {
            var folderSelect = document.getElementById('folder_id');
            var slugInput = document.getElementById('slug');
            var titleInput = document.getElementById('title');
            var form = document.querySelector('form.form-card');

            if (folderSelect) folderSelect.addEventListener('change', updatePathPreview);
            if (slugInput) slugInput.addEventListener('input', updatePathPreview);

            // Only auto-update slug from title for new content
            if (isNewContent && titleInput) {
                titleInput.addEventListener('input', updatePathPreview);
            }

            // Show redirect confirmation modal and return a promise
            function showRedirectModal(oldPath, newPath) {
                return new Promise(function(resolve) {
                    var modal = document.getElementById('redirect-modal');
                    var oldPathEl = document.getElementById('redirect-old-path');
                    var newPathEl = document.getElementById('redirect-new-path');
                    var yesBtn = document.getElementById('redirect-yes-btn');
                    var noBtn = document.getElementById('redirect-no-btn');

                    oldPathEl.textContent = oldPath;
                    newPathEl.textContent = newPath;
                    modal.style.display = 'flex';

                    function cleanup() {
                        modal.style.display = 'none';
                        yesBtn.removeEventListener('click', onYes);
                        noBtn.removeEventListener('click', onNo);
                    }

                    function onYes() {
                        cleanup();
                        resolve(true);
                    }

                    function onNo() {
                        cleanup();
                        resolve(false);
                    }

                    yesBtn.addEventListener('click', onYes);
                    noBtn.addEventListener('click', onNo);
                });
            }

            // Show version comment modal and return a promise
            function showVersionCommentModal(nextVersionNumber) {
                return new Promise(function(resolve) {
                    var modal = document.getElementById('version-comment-modal');
                    var versionDisplay = document.getElementById('version-number-display');
                    var commentInput = document.getElementById('version-comment-input');
                    var saveBtn = document.getElementById('version-comment-save');
                    var cancelBtn = document.getElementById('version-comment-cancel');

                    if (!modal) {
                        // Modal doesn't exist (probably new content)
                        resolve({ proceed: true, comment: '' });
                        return;
                    }

                    versionDisplay.textContent = 'v' + nextVersionNumber;
                    commentInput.value = '';
                    modal.style.display = 'flex';
                    commentInput.focus();

                    function cleanup() {
                        modal.style.display = 'none';
                        saveBtn.removeEventListener('click', onSave);
                        cancelBtn.removeEventListener('click', onCancel);
                        commentInput.removeEventListener('keydown', onKeydown);
                    }

                    function onSave() {
                        cleanup();
                        resolve({ proceed: true, comment: commentInput.value.trim() });
                    }

                    function onCancel() {
                        cleanup();
                        resolve({ proceed: false, comment: '' });
                    }

                    function onKeydown(e) {
                        if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
                            onSave();
                        } else if (e.key === 'Escape') {
                            onCancel();
                        }
                    }

                    saveBtn.addEventListener('click', onSave);
                    cancelBtn.addEventListener('click', onCancel);
                    commentInput.addEventListener('keydown', onKeydown);
                });
            }

            // Validate slug on form submit and prompt for redirect if slug changed
            if (form) {
                form.addEventListener('submit', async function(e) {
                    e.preventDefault();
                    if (await validateSlug()) {
                        // Check if slug has changed (for existing content only)
                        if (!isNewContent && slugRenameEnabled) {
                            var slugInput = document.getElementById('slug');
                            var folderSelect = document.getElementById('folder_id');
                            var newSlug = slugInput ? slugInput.value.trim() : '';

                            var folderPath = '';
                            if (folderSelect && folderSelect.value !== 'root') {
                                var selectedOption = folderSelect.options[folderSelect.selectedIndex];
                                folderPath = selectedOption.text.split(' ')[0];
                            }

                            var newFullPath = folderPath ? folderPath + '/' + newSlug : '/' + newSlug;
                            if (!newSlug) newFullPath = folderPath || '/';

                            var oldFullPath = '{{if .Content}}{{.Content.FullPath}}{{end}}';
                            if (!oldFullPath) oldFullPath = originalSlug ? '/' + originalSlug : '/';

                            // If path actually changed, show redirect modal
                            if (newFullPath !== oldFullPath) {
                                var createRedirect = await showRedirectModal(oldFullPath, newFullPath);
                                document.getElementById('create_redirect').value = createRedirect ? 'yes' : 'no';
                            }
                        }

                        // Show version comment modal for updates (not new content)
                        if (!isNewContent) {
                            var currentVersionCount = {{if .Versions}}{{len .Versions}}{{else}}0{{end}};
                            var nextVersion = currentVersionCount + 1;
                            var result = await showVersionCommentModal(nextVersion);
                            if (!result.proceed) {
                                return; // User cancelled
                            }
                            document.getElementById('version_comment').value = result.comment;
                        }

                        form.submit();
                    }
                });
            }

            // Initial preview
            updatePathPreview();
        });

        // Change template modal functions
        function showChangeTemplateModal() {
            var modal = document.getElementById('change-template-modal');
            if (modal) {
                modal.style.display = 'flex';
            }
        }

        function closeChangeTemplateModal() {
            var modal = document.getElementById('change-template-modal');
            if (modal) {
                modal.style.display = 'none';
            }
        }

        function proceedToTemplatePreview() {
            var select = document.getElementById('new-template-select');
            if (select && select.value) {
                var contentId = '{{if .Content}}{{.Content.ID.Hex}}{{end}}';
                window.location.href = '/cm/content/' + contentId + '/change-template/' + select.value;
            }
        }
        </script>
    ` + adminLayoutEnd,

	"change_template_preview": adminLayoutStart + `
        <div class="page-header">
            <h1>Change Template Preview</h1>
        </div>
        <div class="form-card">
            <div style="margin-bottom: 1.5rem; padding: 1rem; background: var(--bg-tertiary); border-radius: var(--radius);">
                <p style="margin: 0;">
                    <strong>Content:</strong> {{.Content.Title}}<br>
                    <strong>From:</strong> {{.OldTemplate.Name}} → <strong>To:</strong> {{.NewTemplate.Name}}
                </p>
            </div>

            <h3 style="margin-bottom: 1rem; color: var(--text);">Field Mapping</h3>

            {{if .MappedFields}}
            <div style="margin-bottom: 1.5rem;">
                <h4 style="color: var(--success); margin-bottom: 0.75rem;">Fields that will be preserved:</h4>
                <table style="width: 100%;">
                    <thead>
                        <tr>
                            <th style="text-align: left; padding: 0.5rem; border-bottom: 1px solid var(--border);">Field Name</th>
                            <th style="text-align: left; padding: 0.5rem; border-bottom: 1px solid var(--border);">Current Value</th>
                        </tr>
                    </thead>
                    <tbody>
                        {{range .MappedFields}}
                        <tr>
                            <td style="padding: 0.5rem; border-bottom: 1px solid var(--border);">
                                <code>{{.Name}}</code>
                                {{if ne .OldType .NewType}}
                                <span style="color: var(--warning); font-size: 0.85rem;">(type: {{.OldType}} → {{.NewType}})</span>
                                {{end}}
                            </td>
                            <td style="padding: 0.5rem; border-bottom: 1px solid var(--border); max-width: 400px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">
                                {{if .Value}}{{.Value}}{{else}}<span style="color: var(--muted);">(empty)</span>{{end}}
                            </td>
                        </tr>
                        {{end}}
                    </tbody>
                </table>
            </div>
            {{end}}

            {{if .LostFields}}
            <div style="margin-bottom: 1.5rem; padding: 1rem; background: rgba(239, 68, 68, 0.1); border: 1px solid rgba(239, 68, 68, 0.3); border-radius: var(--radius);">
                <h4 style="color: var(--danger); margin-bottom: 0.75rem;">Fields that will be LOST:</h4>
                <p style="color: var(--muted); margin-bottom: 0.75rem;">These fields exist in the current template but not in the new template. Their data will be permanently lost.</p>
                <table style="width: 100%;">
                    <thead>
                        <tr>
                            <th style="text-align: left; padding: 0.5rem; border-bottom: 1px solid rgba(239, 68, 68, 0.3);">Field Name</th>
                            <th style="text-align: left; padding: 0.5rem; border-bottom: 1px solid rgba(239, 68, 68, 0.3);">Current Value (will be lost)</th>
                        </tr>
                    </thead>
                    <tbody>
                        {{range .LostFields}}
                        <tr>
                            <td style="padding: 0.5rem; border-bottom: 1px solid rgba(239, 68, 68, 0.2);"><code>{{.Name}}</code></td>
                            <td style="padding: 0.5rem; border-bottom: 1px solid rgba(239, 68, 68, 0.2); max-width: 400px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--danger);">
                                {{if .Value}}{{.Value}}{{else}}<span style="color: var(--muted);">(empty)</span>{{end}}
                            </td>
                        </tr>
                        {{end}}
                    </tbody>
                </table>
            </div>
            {{end}}

            {{if .NewFields}}
            <div style="margin-bottom: 1.5rem;">
                <h4 style="color: var(--primary); margin-bottom: 0.75rem;">New fields (will start empty):</h4>
                <ul style="margin: 0; padding-left: 1.5rem;">
                    {{range .NewFields}}
                    <li><code>{{.Name}}</code> ({{.Type}})</li>
                    {{end}}
                </ul>
            </div>
            {{end}}

            <div style="padding: 1rem; background: rgba(245, 158, 11, 0.1); border: 1px solid rgba(245, 158, 11, 0.3); border-radius: var(--radius); margin-bottom: 1.5rem;">
                <p style="margin: 0; color: var(--warning);">
                    <strong>Note:</strong> This action will save a new version of the content with the template change. You can revert to a previous version if needed.
                </p>
            </div>

            <div class="form-actions">
                <a href="/cm/content/{{.Content.ID.Hex}}" class="btn btn-outline">Cancel</a>
                <form method="POST" action="/cm/content/{{.Content.ID.Hex}}/change-template/{{.NewTemplate.ID.Hex}}/confirm" style="display: inline;">
            {{.CSRFField}}
                    <button type="submit" class="btn" style="background: var(--warning); color: white;">Confirm Template Change</button>
                </form>
            </div>
        </div>
    ` + adminLayoutEnd,

	"collections_list": adminLayoutStart + `
        <div class="page-header">
            <h1>Collections</h1>
            <a href="/cm/collections/new" class="btn btn-primary">New Collection</a>
        </div>
        <div class="table-container">
            <table>
                <thead>
                    <tr>
                        <th>Name</th>
                        <th>Category Filter</th>
                        <th>Sort</th>
                        <th>Actions</th>
                    </tr>
                </thead>
                <tbody>
                    {{range .Collections}}
                    <tr>
                        <td><strong>{{.Name}}</strong><br><small>/{{.Slug}}</small></td>
                        <td>{{.Category}}</td>
                        <td>{{.SortField}} ({{.SortOrder}})</td>
                        <td class="actions">
                            <a href="/{{.Slug}}" target="_blank" class="btn btn-sm btn-outline">View</a>
                            <a href="/cm/collections/{{.ID.Hex}}" class="btn btn-sm">Edit</a>
                            <form method="POST" action="/cm/collections/{{.ID.Hex}}/delete" onsubmit="return confirmDelete(this, 'Are you sure you want to delete this collection?')">
            {{$.CSRFField}}
                                <button type="submit" class="btn btn-sm btn-danger">Delete</button>
                            </form>
                        </td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>
    ` + adminLayoutEnd,

	"collection_form": adminLayoutStart + `
        <div class="page-header">
            <h1>{{if .IsNew}}New Collection{{else}}Edit Collection{{end}}</h1>
        </div>
        {{if .Error}}<div class="error-message">{{.Error}}</div>{{end}}
        <form method="POST" class="form-card">
            {{.CSRFField}}
            <div class="form-group">
                <label for="name">Collection Name</label>
                <input type="text" id="name" name="name" value="{{if .Collection}}{{.Collection.Name}}{{end}}" required>
            </div>
            <div class="form-group">
                <label for="description">Description</label>
                <textarea id="description" name="description" rows="2">{{if .Collection}}{{.Collection.Description}}{{end}}</textarea>
            </div>
            <div class="form-row">
                <div class="form-group">
                    <label for="category">Category Filter</label>
                    <input type="text" id="category" name="category" value="{{if .Collection}}{{.Collection.Category}}{{end}}" placeholder="e.g., blog">
                </div>
                <div class="form-group">
                    <label for="items_per_page">Items Per Page</label>
                    <input type="number" id="items_per_page" name="items_per_page" value="{{if .Collection}}{{.Collection.ItemsPerPage}}{{else}}10{{end}}">
                </div>
            </div>
            <div class="form-row">
                <div class="form-group">
                    <label for="sort_field">Sort Field</label>
                    <input type="text" id="sort_field" name="sort_field" value="{{if .Collection}}{{.Collection.SortField}}{{else}}published_at{{end}}">
                </div>
                <div class="form-group">
                    <label for="sort_order">Sort Order</label>
                    <select id="sort_order" name="sort_order">
                        <option value="desc" {{if .Collection}}{{if eq .Collection.SortOrder "desc"}}selected{{end}}{{end}}>Descending (newest first)</option>
                        <option value="asc" {{if .Collection}}{{if eq .Collection.SortOrder "asc"}}selected{{end}}{{end}}>Ascending (oldest first)</option>
                    </select>
                </div>
            </div>
            <div class="form-group">
                <label for="item_template">Item Template</label>
                <p class="help-text">HTML template for each item. Use {{.field_name}} placeholders.</p>
                <textarea id="item_template" name="item_template" rows="10" class="code-editor">{{if .Collection}}{{.Collection.ItemTemplate}}{{end}}</textarea>
            </div>
            <div class="form-group">
                <label for="page_template">Page Template</label>
                <p class="help-text">Overall page template. Use {{.collection_name}}, {{.collection_description}}, {{.items}}, {{.pagination}}</p>
                <textarea id="page_template" name="page_template" rows="10" class="code-editor">{{if .Collection}}{{.Collection.PageTemplate}}{{end}}</textarea>
            </div>

            <div class="form-actions">
                <a href="/cm/collections" class="btn btn-outline">Cancel</a>
                <button type="submit" class="btn btn-primary">{{if .IsNew}}Create Collection{{else}}Update Collection{{end}}</button>
            </div>
        </form>
    ` + adminLayoutEnd,

	"theme": adminLayoutStart + `
        <div class="page-header">
            <h1>Theme Settings</h1>
            <a href="/cm/theme/versions" class="btn btn-outline">Version History</a>
        </div>
        {{if .Error}}<div class="error-message">{{.Error}}</div>{{end}}
        {{if .Success}}<div class="success-message">{{.Success}}</div>{{end}}
        <form method="POST" class="form-card">
            {{.CSRFField}}
            <div class="form-section">
                <h3>Site Identity</h3>
                <div class="form-row">
                    <div class="form-group">
                        <label for="site_name">Site Name</label>
                        <input type="text" id="site_name" name="site_name" value="{{.Settings.SiteName}}">
                    </div>
                    <div class="form-group">
                        <label for="site_tagline">Tagline</label>
                        <input type="text" id="site_tagline" name="site_tagline" value="{{.Settings.SiteTagline}}">
                    </div>
                </div>
                <div class="form-group" style="margin-top: 1rem;">
                    <label for="logo_url">Logo URL (optional)</label>
                    <input type="text" id="logo_url" name="logo_url" value="{{.Settings.LogoURL}}" placeholder="/assets/images/logo.png">
                    <small style="color: var(--text-muted);">Upload images in <a href="/cm/assets">Asset Library</a>, then paste the URL here</small>
                </div>
            </div>

            <div class="form-section">
                <h3>Colors</h3>
                <div class="color-grid">
                    <div class="form-group">
                        <label for="primary_color">Primary</label>
                        <input type="color" id="primary_color" name="primary_color" value="{{.Settings.PrimaryColor}}">
                    </div>
                    <div class="form-group">
                        <label for="secondary_color">Secondary</label>
                        <input type="color" id="secondary_color" name="secondary_color" value="{{.Settings.SecondaryColor}}">
                    </div>
                    <div class="form-group">
                        <label for="accent_color">Accent</label>
                        <input type="color" id="accent_color" name="accent_color" value="{{.Settings.AccentColor}}">
                    </div>
                    <div class="form-group">
                        <label for="background_color">Background</label>
                        <input type="color" id="background_color" name="background_color" value="{{.Settings.BackgroundColor}}">
                    </div>
                    <div class="form-group">
                        <label for="text_color">Text</label>
                        <input type="color" id="text_color" name="text_color" value="{{.Settings.TextColor}}">
                    </div>
                </div>
            </div>

            <div class="form-section">
                <h3>Typography</h3>
                <div class="form-row">
                    <div class="form-group">
                        <label for="font_family">Body Font</label>
                        <input type="text" id="font_family" name="font_family" value="{{.Settings.FontFamily}}">
                    </div>
                    <div class="form-group">
                        <label for="heading_font">Heading Font</label>
                        <input type="text" id="heading_font" name="heading_font" value="{{.Settings.HeadingFont}}">
                    </div>
                    <div class="form-group">
                        <label for="border_radius">Border Radius</label>
                        <input type="text" id="border_radius" name="border_radius" value="{{.Settings.BorderRadius}}" placeholder="e.g., 12px">
                    </div>
                </div>
            </div>

            <div class="form-section">
                <h3>Custom CSS</h3>
                <p class="help-text">Injected into the &lt;style&gt; tag in the page &lt;head&gt;, after the theme CSS variables.</p>
                <div class="form-group">
                    <textarea id="custom_css" name="custom_css" rows="10" class="code-editor">{{.Settings.CustomCSS}}</textarea>
                </div>
            </div>

            <div class="form-section">
                <h3>Head HTML</h3>
                <p class="help-text">Injected immediately after the opening &lt;head&gt; tag on every page (for analytics, scripts, meta tags, etc.).</p>
                <div class="form-group">
                    <textarea id="head_html" name="head_html" rows="8" class="code-editor" placeholder="<!-- Google Analytics, meta tags, external scripts, etc. -->">{{.Settings.HeadHTML}}</textarea>
                </div>
            </div>

            <div class="form-section">
                <h3>Site Header</h3>
                <p class="help-text">Injected at the start of the &lt;body&gt; on pages with header enabled (navigation, logo, etc.).</p>
                <div class="form-group">
                    <textarea id="header_html" name="header_html" rows="12" class="code-editor">{{.Settings.HeaderHTML}}</textarea>
                </div>
            </div>

            <div class="form-section">
                <h3>Site Footer</h3>
                <p class="help-text">Injected at the end of the &lt;body&gt; on pages with footer enabled (copyright, links, etc.).</p>
                <div class="form-group">
                    <textarea id="footer_html" name="footer_html" rows="15" class="code-editor">{{.Settings.FooterHTML}}</textarea>
                </div>
            </div>

            <div class="form-actions">
                <button type="submit" class="btn btn-primary">Save Theme</button>
            </div>
        </form>

        <link href="https://cdn.jsdelivr.net/npm/quill@2.0.3/dist/quill.snow.css" rel="stylesheet">
        <script src="https://cdn.jsdelivr.net/npm/quill@2.0.3/dist/quill.js"></script>
        <style>
            .quill-wrapper { margin-bottom: 1rem; }
            .quill-wrapper .ql-toolbar { background: rgba(15, 23, 42, 0.5); border-color: var(--border); border-radius: var(--radius) var(--radius) 0 0; }
            .quill-wrapper .ql-container { background: rgba(15, 23, 42, 0.5); border-color: var(--border); border-radius: 0 0 var(--radius) var(--radius); min-height: 150px; }
            .quill-wrapper .ql-editor { color: var(--text); min-height: 130px; font-size: 1rem; }
            .quill-wrapper .ql-editor.ql-blank::before { color: var(--text-muted); }
            .ql-toolbar .ql-stroke { stroke: var(--text); }
            .ql-toolbar .ql-fill { fill: var(--text); }
            .ql-toolbar .ql-picker { color: var(--text); }
            .ql-toolbar .ql-picker-options { background: var(--bg-card); border-color: var(--border); }
            .ql-toolbar button:hover, .ql-toolbar button.ql-active { color: var(--primary); }
            .ql-toolbar button:hover .ql-stroke, .ql-toolbar button.ql-active .ql-stroke { stroke: var(--primary); }

            /* Custom link modal styles */
            .link-modal-overlay {
                position: fixed;
                top: 0;
                left: 0;
                right: 0;
                bottom: 0;
                background: rgba(0, 0, 0, 0.7);
                display: flex;
                align-items: center;
                justify-content: center;
                z-index: 10000;
            }
            .link-modal {
                background: var(--bg-card);
                border: 1px solid var(--border);
                border-radius: var(--radius);
                padding: 1.5rem;
                width: 100%;
                max-width: 500px;
                box-shadow: 0 20px 50px rgba(0, 0, 0, 0.5);
            }
            .link-modal h3 {
                margin-bottom: 1.5rem;
                font-size: 1.25rem;
                color: var(--text);
            }
            .link-type-tabs {
                display: flex;
                gap: 0.5rem;
                margin-bottom: 1.5rem;
            }
            .link-type-tab {
                flex: 1;
                padding: 0.75rem;
                border: 1px solid var(--border);
                border-radius: var(--radius);
                background: transparent;
                color: var(--text-muted);
                cursor: pointer;
                font-size: 0.9rem;
                transition: all 0.2s;
            }
            .link-type-tab:hover {
                border-color: var(--primary);
                color: var(--text);
            }
            .link-type-tab.active {
                background: linear-gradient(135deg, var(--primary), var(--secondary));
                border-color: transparent;
                color: white;
            }
            .link-input-group {
                margin-bottom: 1rem;
                position: relative;
            }
            .link-input-group label {
                display: block;
                margin-bottom: 0.5rem;
                color: var(--text);
                font-weight: 500;
            }
            .link-input-group input {
                width: 100%;
                padding: 0.75rem 1rem;
                background: rgba(15, 23, 42, 0.5);
                border: 1px solid var(--border);
                border-radius: var(--radius);
                color: var(--text);
                font-size: 1rem;
            }
            .link-input-group input:focus {
                outline: none;
                border-color: var(--primary);
                box-shadow: 0 0 0 3px rgba(99, 102, 241, 0.2);
            }
            .autocomplete-dropdown {
                position: absolute;
                top: 100%;
                left: 0;
                right: 0;
                background: var(--bg-card);
                border: 1px solid var(--border);
                border-top: none;
                border-radius: 0 0 var(--radius) var(--radius);
                max-height: 200px;
                overflow-y: auto;
                z-index: 10001;
                display: none;
            }
            .autocomplete-dropdown.show {
                display: block;
            }
            .autocomplete-item {
                padding: 0.75rem 1rem;
                cursor: pointer;
                border-bottom: 1px solid var(--border);
            }
            .autocomplete-item:last-child {
                border-bottom: none;
            }
            .autocomplete-item:hover,
            .autocomplete-item.selected {
                background: var(--bg-hover);
            }
            .autocomplete-item .slug {
                font-family: 'JetBrains Mono', monospace;
                font-size: 0.85rem;
                color: var(--accent);
            }
            .autocomplete-item .title {
                font-size: 0.8rem;
                color: var(--text-muted);
                margin-top: 0.25rem;
            }
            .link-modal-actions {
                display: flex;
                gap: 1rem;
                justify-content: flex-end;
                margin-top: 1.5rem;
                padding-top: 1rem;
                border-top: 1px solid var(--border);
            }
            .link-hint {
                font-size: 0.8rem;
                color: var(--text-muted);
                margin-top: 0.5rem;
            }
        </style>
        <script>
        // Global slugs cache for autocomplete
        var siteSlugs = [];

        // Load all slugs for autocomplete
        function loadSlugs() {
            fetch('/api/slugs')
                .then(function(res) { return res.json(); })
                .then(function(data) { siteSlugs = data; })
                .catch(function(err) { console.log('Failed to load slugs:', err); });
        }

        // Custom link handler
        function createLinkModal(quill, existingLink) {
            var selection = quill.getSelection();
            var selectedText = selection ? quill.getText(selection.index, selection.length) : '';

            var overlay = document.createElement('div');
            overlay.className = 'link-modal-overlay';

            var modal = document.createElement('div');
            modal.className = 'link-modal';
            modal.innerHTML = ` + "`" + `
                <h3>${existingLink ? 'Edit Link' : 'Insert Link'}</h3>
                <div class="link-type-tabs">
                    <button type="button" class="link-type-tab active" data-type="internal">Internal Page</button>
                    <button type="button" class="link-type-tab" data-type="external">External URL</button>
                </div>
                <div id="internal-link-section">
                    <div class="link-input-group">
                        <label>Select Page</label>
                        <input type="text" id="internal-link-input" placeholder="Start typing to search pages..." autocomplete="off">
                        <div class="autocomplete-dropdown" id="autocomplete-dropdown"></div>
                        <p class="link-hint">Type to search, use Tab to accept suggestion</p>
                    </div>
                </div>
                <div id="external-link-section" style="display: none;">
                    <div class="link-input-group">
                        <label>URL</label>
                        <input type="text" id="external-link-input" placeholder="https://example.com">
                    </div>
                </div>
                <div class="link-modal-actions">
                    <button type="button" class="btn btn-outline" id="link-cancel">Cancel</button>
                    ${existingLink ? '<button type="button" class="btn btn-danger" id="link-remove">Remove Link</button>' : ''}
                    <button type="button" class="btn btn-primary" id="link-save">Save</button>
                </div>
            ` + "`" + `;

            overlay.appendChild(modal);
            document.body.appendChild(overlay);

            var internalInput = modal.querySelector('#internal-link-input');
            var externalInput = modal.querySelector('#external-link-input');
            var dropdown = modal.querySelector('#autocomplete-dropdown');
            var currentLinkType = 'internal';
            var selectedIndex = -1;
            var filteredSlugs = [];

            // Pre-fill with existing link
            if (existingLink) {
                if (existingLink.startsWith('/')) {
                    internalInput.value = existingLink;
                    currentLinkType = 'internal';
                } else {
                    externalInput.value = existingLink;
                    currentLinkType = 'external';
                    modal.querySelector('[data-type="internal"]').classList.remove('active');
                    modal.querySelector('[data-type="external"]').classList.add('active');
                    modal.querySelector('#internal-link-section').style.display = 'none';
                    modal.querySelector('#external-link-section').style.display = 'block';
                }
            }

            // Tab switching
            modal.querySelectorAll('.link-type-tab').forEach(function(tab) {
                tab.addEventListener('click', function() {
                    modal.querySelectorAll('.link-type-tab').forEach(function(t) { t.classList.remove('active'); });
                    tab.classList.add('active');
                    currentLinkType = tab.dataset.type;
                    if (currentLinkType === 'internal') {
                        modal.querySelector('#internal-link-section').style.display = 'block';
                        modal.querySelector('#external-link-section').style.display = 'none';
                        internalInput.focus();
                    } else {
                        modal.querySelector('#internal-link-section').style.display = 'none';
                        modal.querySelector('#external-link-section').style.display = 'block';
                        externalInput.focus();
                    }
                });
            });

            // Autocomplete functionality
            function updateAutocomplete() {
                var query = internalInput.value.toLowerCase();
                filteredSlugs = siteSlugs.filter(function(item) {
                    return item.slug.toLowerCase().includes(query) || item.title.toLowerCase().includes(query);
                }).slice(0, 10);

                if (filteredSlugs.length > 0 && query.length > 0) {
                    dropdown.innerHTML = filteredSlugs.map(function(item, i) {
                        return '<div class="autocomplete-item' + (i === selectedIndex ? ' selected' : '') + '" data-slug="' + item.slug + '">' +
                            '<div class="slug">' + item.slug + '</div>' +
                            '<div class="title">' + item.title + '</div>' +
                        '</div>';
                    }).join('');
                    dropdown.classList.add('show');
                } else {
                    dropdown.classList.remove('show');
                }
            }

            internalInput.addEventListener('input', function() {
                selectedIndex = -1;
                updateAutocomplete();
            });

            internalInput.addEventListener('keydown', function(e) {
                if (e.key === 'Tab' && filteredSlugs.length > 0) {
                    e.preventDefault();
                    var idx = selectedIndex >= 0 ? selectedIndex : 0;
                    internalInput.value = filteredSlugs[idx].slug;
                    dropdown.classList.remove('show');
                } else if (e.key === 'ArrowDown') {
                    e.preventDefault();
                    selectedIndex = Math.min(selectedIndex + 1, filteredSlugs.length - 1);
                    updateAutocomplete();
                } else if (e.key === 'ArrowUp') {
                    e.preventDefault();
                    selectedIndex = Math.max(selectedIndex - 1, -1);
                    updateAutocomplete();
                } else if (e.key === 'Enter') {
                    e.preventDefault();
                    if (selectedIndex >= 0 && filteredSlugs[selectedIndex]) {
                        internalInput.value = filteredSlugs[selectedIndex].slug;
                        dropdown.classList.remove('show');
                    } else {
                        modal.querySelector('#link-save').click();
                    }
                } else if (e.key === 'Escape') {
                    overlay.remove();
                }
            });

            dropdown.addEventListener('click', function(e) {
                var item = e.target.closest('.autocomplete-item');
                if (item) {
                    internalInput.value = item.dataset.slug;
                    dropdown.classList.remove('show');
                }
            });

            // Close dropdown when clicking outside
            document.addEventListener('click', function closeDropdown(e) {
                if (!dropdown.contains(e.target) && e.target !== internalInput) {
                    dropdown.classList.remove('show');
                }
            });

            // Save link
            modal.querySelector('#link-save').addEventListener('click', function() {
                var link = currentLinkType === 'internal' ? internalInput.value : externalInput.value;
                if (link) {
                    // Ensure internal links start with /
                    if (currentLinkType === 'internal' && !link.startsWith('/')) {
                        link = '/' + link;
                    }
                    if (selection && selection.length > 0) {
                        quill.formatText(selection.index, selection.length, 'link', link);
                    } else {
                        // Insert link at cursor position
                        var text = selectedText || link;
                        quill.insertText(selection ? selection.index : 0, text, 'link', link);
                    }
                }
                overlay.remove();
            });

            // Cancel
            modal.querySelector('#link-cancel').addEventListener('click', function() {
                overlay.remove();
            });

            // Remove link
            var removeBtn = modal.querySelector('#link-remove');
            if (removeBtn) {
                removeBtn.addEventListener('click', function() {
                    if (selection && selection.length > 0) {
                        quill.formatText(selection.index, selection.length, 'link', false);
                    }
                    overlay.remove();
                });
            }

            // Close on overlay click
            overlay.addEventListener('click', function(e) {
                if (e.target === overlay) {
                    overlay.remove();
                }
            });

            // Focus appropriate input
            setTimeout(function() {
                if (currentLinkType === 'internal') {
                    internalInput.focus();
                } else {
                    externalInput.focus();
                }
            }, 100);
        }

        // Initialize Quill with custom link handler
        function initQuillEditor(textarea) {
            var wrapper = document.createElement('div');
            wrapper.className = 'quill-wrapper';
            var editorDiv = document.createElement('div');
            wrapper.appendChild(editorDiv);
            textarea.parentNode.insertBefore(wrapper, textarea);
            textarea.style.display = 'none';

            var quill = new Quill(editorDiv, {
                theme: 'snow',
                modules: {
                    toolbar: {
                        container: [
                            [{ 'header': [1, 2, 3, false] }],
                            ['bold', 'italic', 'underline', 'strike'],
                            [{ 'color': [] }, { 'background': [] }],
                            [{ 'list': 'ordered'}, { 'list': 'bullet' }],
                            [{ 'indent': '-1'}, { 'indent': '+1' }],
                            ['link', 'image', 'video'],
                            ['blockquote', 'code-block'],
                            ['clean']
                        ],
                        handlers: {
                            'link': function(value) {
                                var selection = quill.getSelection();
                                var existingLink = null;
                                if (selection && selection.length > 0) {
                                    var format = quill.getFormat(selection);
                                    existingLink = format.link || null;
                                }
                                createLinkModal(quill, existingLink);
                            }
                        }
                    }
                }
            });

            quill.root.innerHTML = textarea.value;

            quill.on('text-change', function() {
                textarea.value = quill.root.innerHTML;
            });

            textarea.form.addEventListener('submit', function() {
                textarea.value = quill.root.innerHTML;
            });

            // Store quill instance on wrapper for later access
            wrapper.quillInstance = quill;

            return quill;
        }

        document.addEventListener('DOMContentLoaded', function() {
            // Load slugs for autocomplete
            loadSlugs();

            document.querySelectorAll('.richtext').forEach(function(textarea) {
                initQuillEditor(textarea);
            });
        });
        </script>
    ` + adminLayoutEnd,

	"theme_versions": adminLayoutStart + `
        <div class="page-header">
            <h1>Theme Version History</h1>
            <a href="/cm/theme" class="btn btn-outline">← Back to Theme</a>
        </div>
        <p class="page-subtitle">View and restore previous theme configurations. Each change to the theme settings creates a new version.</p>
        {{if not .Versions}}
        <div class="empty-state">
            <p>No theme versions yet. Make changes to the theme to create version history.</p>
        </div>
        {{else}}
        <div class="table-container">
            <table>
                <thead>
                    <tr>
                        <th>Version</th>
                        <th>Site Name</th>
                        <th>Comment</th>
                        <th>Created</th>
                        <th>Actions</th>
                    </tr>
                </thead>
                <tbody>
                    {{range $i, $v := .Versions}}
                    <tr{{if eq $i 0}} class="current-version"{{end}}>
                        <td>
                            <strong>v{{$v.Version}}</strong>
                            {{if eq $i 0}}<span class="badge badge-success">Current</span>{{end}}
                        </td>
                        <td>{{$v.SiteName}}</td>
                        <td>{{if $v.Comment}}{{$v.Comment}}{{else}}<span class="text-muted">—</span>{{end}}</td>
                        <td>{{$v.CreatedAt.Format "Jan 2, 2006 3:04 PM"}}</td>
                        <td class="actions">
                            <a href="/cm/theme/versions/{{$v.Version}}" class="btn btn-sm">View Diff</a>
                            {{if ne $i 0}}
                            <form method="POST" action="/cm/theme/versions/{{$v.Version}}/revert" style="display:inline" onsubmit="return confirmRevert(this, {{$v.Version}})">
                                {{$.CSRFField}}
                                <button type="submit" class="btn btn-sm btn-primary">Revert</button>
                            </form>
                            {{end}}
                        </td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>
        {{end}}
        <style>
            .empty-state {
                background: var(--bg-card);
                border: 1px dashed var(--border);
                border-radius: var(--radius);
                padding: 3rem;
                text-align: center;
                color: var(--text-muted);
            }
            .current-version {
                background: rgba(99, 102, 241, 0.1);
            }
            .badge {
                display: inline-block;
                padding: 0.2rem 0.5rem;
                border-radius: 4px;
                font-size: 0.7rem;
                font-weight: 600;
                text-transform: uppercase;
                margin-left: 0.5rem;
            }
            .badge-success {
                background: rgba(16, 185, 129, 0.2);
                color: #10b981;
            }
            .text-muted {
                color: var(--text-muted);
            }
        </style>
        <script>
        function confirmRevert(form, version) {
            event.preventDefault();

            var overlay = document.createElement('div');
            overlay.className = 'modal-overlay';
            overlay.innerHTML = '<div class="modal-box">' +
                '<h3>Revert Theme to Version ' + version + '?</h3>' +
                '<p style="color: var(--text-muted); margin-bottom: 1.5rem;">This will restore the theme settings from version ' + version + '. A new version will be created with the restored settings.</p>' +
                '<div class="modal-actions">' +
                '<button type="button" class="btn btn-outline" onclick="this.closest(\'.modal-overlay\').remove()">Cancel</button>' +
                '<button type="button" class="btn btn-primary" id="confirmRevertBtn">Revert Theme</button>' +
                '</div></div>';
            document.body.appendChild(overlay);

            document.getElementById('confirmRevertBtn').addEventListener('click', function() {
                form.submit();
            });

            overlay.addEventListener('click', function(e) {
                if (e.target === overlay) overlay.remove();
            });

            return false;
        }
        </script>
    ` + adminLayoutEnd,

	"theme_version_diff": adminLayoutStart + `
        <div class="page-header">
            <h1>Theme Version Comparison</h1>
            <a href="/cm/theme/versions" class="btn btn-outline">← Back to Versions</a>
        </div>
        <p style="color: var(--muted); margin-bottom: 2rem;">
            Comparing <strong>Version {{.Version.Version}}</strong> (saved {{.Version.CreatedAt.Format "Jan 2, 2006 3:04 PM"}})
            with <strong>Current Theme</strong>
        </p>

        <div class="diff-container">
            <!-- Site Identity -->
            <div class="diff-section" data-field="site_name">
                <h3>Site Name <span class="diff-badge"></span></h3>
                <div class="diff-row">
                    <div class="diff-col diff-old">
                        <div class="diff-label">Version {{.Version.Version}}</div>
                        <div class="diff-content" data-old="{{.Version.SiteName}}">{{.Version.SiteName}}</div>
                    </div>
                    <div class="diff-col diff-new">
                        <div class="diff-label">Current</div>
                        <div class="diff-content" data-new="{{.Current.SiteName}}">{{.Current.SiteName}}</div>
                    </div>
                </div>
            </div>

            <div class="diff-section" data-field="site_tagline">
                <h3>Tagline <span class="diff-badge"></span></h3>
                <div class="diff-row">
                    <div class="diff-col diff-old">
                        <div class="diff-label">Version {{.Version.Version}}</div>
                        <div class="diff-content" data-old="{{.Version.SiteTagline}}">{{.Version.SiteTagline}}</div>
                    </div>
                    <div class="diff-col diff-new">
                        <div class="diff-label">Current</div>
                        <div class="diff-content" data-new="{{.Current.SiteTagline}}">{{.Current.SiteTagline}}</div>
                    </div>
                </div>
            </div>

            <!-- Colors -->
            <div class="diff-section" data-field="colors">
                <h3>Colors <span class="diff-badge"></span></h3>
                <div class="diff-row">
                    <div class="diff-col diff-old">
                        <div class="diff-label">Version {{.Version.Version}}</div>
                        <div class="diff-content" data-old="{{.Version.PrimaryColor}}|{{.Version.SecondaryColor}}|{{.Version.AccentColor}}|{{.Version.BackgroundColor}}|{{.Version.TextColor}}">
                            <div class="color-row"><span class="color-swatch" style="background: {{.Version.PrimaryColor}}"></span> Primary: {{.Version.PrimaryColor}}</div>
                            <div class="color-row"><span class="color-swatch" style="background: {{.Version.SecondaryColor}}"></span> Secondary: {{.Version.SecondaryColor}}</div>
                            <div class="color-row"><span class="color-swatch" style="background: {{.Version.AccentColor}}"></span> Accent: {{.Version.AccentColor}}</div>
                            <div class="color-row"><span class="color-swatch" style="background: {{.Version.BackgroundColor}}"></span> Background: {{.Version.BackgroundColor}}</div>
                            <div class="color-row"><span class="color-swatch" style="background: {{.Version.TextColor}}"></span> Text: {{.Version.TextColor}}</div>
                        </div>
                    </div>
                    <div class="diff-col diff-new">
                        <div class="diff-label">Current</div>
                        <div class="diff-content" data-new="{{.Current.PrimaryColor}}|{{.Current.SecondaryColor}}|{{.Current.AccentColor}}|{{.Current.BackgroundColor}}|{{.Current.TextColor}}">
                            <div class="color-row"><span class="color-swatch" style="background: {{.Current.PrimaryColor}}"></span> Primary: {{.Current.PrimaryColor}}</div>
                            <div class="color-row"><span class="color-swatch" style="background: {{.Current.SecondaryColor}}"></span> Secondary: {{.Current.SecondaryColor}}</div>
                            <div class="color-row"><span class="color-swatch" style="background: {{.Current.AccentColor}}"></span> Accent: {{.Current.AccentColor}}</div>
                            <div class="color-row"><span class="color-swatch" style="background: {{.Current.BackgroundColor}}"></span> Background: {{.Current.BackgroundColor}}</div>
                            <div class="color-row"><span class="color-swatch" style="background: {{.Current.TextColor}}"></span> Text: {{.Current.TextColor}}</div>
                        </div>
                    </div>
                </div>
            </div>

            <!-- Typography -->
            <div class="diff-section" data-field="typography">
                <h3>Typography <span class="diff-badge"></span></h3>
                <div class="diff-row">
                    <div class="diff-col diff-old">
                        <div class="diff-label">Version {{.Version.Version}}</div>
                        <div class="diff-content" data-old="{{.Version.FontFamily}}|{{.Version.HeadingFont}}|{{.Version.BorderRadius}}">
                            <p>Body Font: {{.Version.FontFamily}}</p>
                            <p>Heading Font: {{.Version.HeadingFont}}</p>
                            <p>Border Radius: {{.Version.BorderRadius}}</p>
                        </div>
                    </div>
                    <div class="diff-col diff-new">
                        <div class="diff-label">Current</div>
                        <div class="diff-content" data-new="{{.Current.FontFamily}}|{{.Current.HeadingFont}}|{{.Current.BorderRadius}}">
                            <p>Body Font: {{.Current.FontFamily}}</p>
                            <p>Heading Font: {{.Current.HeadingFont}}</p>
                            <p>Border Radius: {{.Current.BorderRadius}}</p>
                        </div>
                    </div>
                </div>
            </div>

            <!-- Custom CSS -->
            <div class="diff-section diff-field-section" data-field="custom_css">
                <h3>Custom CSS <span class="diff-badge"></span></h3>
                <div class="diff-row">
                    <div class="diff-col diff-old">
                        <div class="diff-label">Version {{.Version.Version}}</div>
                        <div class="diff-content diff-code" data-old="{{.Version.CustomCSS}}"><pre>{{.Version.CustomCSS}}</pre></div>
                    </div>
                    <div class="diff-col diff-new">
                        <div class="diff-label">Current</div>
                        <div class="diff-content diff-code" data-new="{{.Current.CustomCSS}}"><pre>{{.Current.CustomCSS}}</pre></div>
                    </div>
                </div>
            </div>

            <!-- Head HTML -->
            <div class="diff-section diff-field-section" data-field="head_html">
                <h3>Head HTML <span class="diff-badge"></span></h3>
                <div class="diff-row">
                    <div class="diff-col diff-old">
                        <div class="diff-label">Version {{.Version.Version}}</div>
                        <div class="diff-content diff-code" data-old="{{.Version.HeadHTML}}"><pre>{{.Version.HeadHTML}}</pre></div>
                    </div>
                    <div class="diff-col diff-new">
                        <div class="diff-label">Current</div>
                        <div class="diff-content diff-code" data-new="{{.Current.HeadHTML}}"><pre>{{.Current.HeadHTML}}</pre></div>
                    </div>
                </div>
            </div>

            <!-- Header HTML -->
            <div class="diff-section diff-field-section" data-field="header_html">
                <h3>Header HTML <span class="diff-badge"></span></h3>
                <div class="diff-row">
                    <div class="diff-col diff-old">
                        <div class="diff-label">Version {{.Version.Version}}</div>
                        <div class="diff-content diff-code" data-old="{{.Version.HeaderHTML}}"><pre>{{.Version.HeaderHTML}}</pre></div>
                    </div>
                    <div class="diff-col diff-new">
                        <div class="diff-label">Current</div>
                        <div class="diff-content diff-code" data-new="{{.Current.HeaderHTML}}"><pre>{{.Current.HeaderHTML}}</pre></div>
                    </div>
                </div>
            </div>

            <!-- Footer HTML -->
            <div class="diff-section diff-field-section" data-field="footer_html">
                <h3>Footer HTML <span class="diff-badge"></span></h3>
                <div class="diff-row">
                    <div class="diff-col diff-old">
                        <div class="diff-label">Version {{.Version.Version}}</div>
                        <div class="diff-content diff-code" data-old="{{.Version.FooterHTML}}"><pre>{{.Version.FooterHTML}}</pre></div>
                    </div>
                    <div class="diff-col diff-new">
                        <div class="diff-label">Current</div>
                        <div class="diff-content diff-code" data-new="{{.Current.FooterHTML}}"><pre>{{.Current.FooterHTML}}</pre></div>
                    </div>
                </div>
            </div>
        </div>

        <div class="form-actions" style="margin-top: 2rem;">
            <a href="/cm/theme/versions" class="btn btn-outline">Back to Versions</a>
            <form method="POST" action="/cm/theme/versions/{{.Version.Version}}/revert" style="display:inline" onsubmit="return confirmRevert(this, {{.Version.Version}})">
            {{.CSRFField}}
                <button type="submit" class="btn btn-primary">Revert to Version {{.Version.Version}}</button>
            </form>
        </div>

        <style>
            .diff-container {
                display: flex;
                flex-direction: column;
                gap: 1.5rem;
            }
            .diff-section {
                background: var(--card-bg);
                border: 1px solid var(--border);
                border-radius: var(--radius);
                padding: 1rem;
            }
            .diff-section.has-changes {
                border-color: #f59e0b;
            }
            .diff-section.no-changes {
                opacity: 0.6;
            }
            .diff-section h3 {
                margin: 0 0 1rem 0;
                font-size: 1rem;
                color: var(--accent);
                border-bottom: 1px solid var(--border);
                padding-bottom: 0.5rem;
                display: flex;
                align-items: center;
                gap: 0.75rem;
            }
            .diff-badge {
                font-size: 0.7rem;
                padding: 0.15rem 0.5rem;
                border-radius: 4px;
                text-transform: uppercase;
                font-weight: 600;
            }
            .diff-badge.changed {
                background: rgba(245, 158, 11, 0.2);
                color: #f59e0b;
            }
            .diff-badge.unchanged {
                background: rgba(107, 114, 128, 0.2);
                color: #6b7280;
            }
            .diff-row {
                display: grid;
                grid-template-columns: 1fr 1fr;
                gap: 1rem;
            }
            .diff-col {
                min-width: 0;
            }
            .diff-label {
                font-size: 0.75rem;
                text-transform: uppercase;
                color: var(--muted);
                margin-bottom: 0.5rem;
                font-weight: 600;
            }
            .diff-old .diff-label {
                color: #f59e0b;
            }
            .diff-new .diff-label {
                color: #10b981;
            }
            .diff-content {
                background: rgba(15, 23, 42, 0.5);
                border: 1px solid var(--border);
                border-radius: 4px;
                padding: 0.75rem;
                font-size: 0.9rem;
                overflow-x: auto;
                max-height: 400px;
                overflow-y: auto;
            }
            .diff-old .diff-content {
                border-left: 3px solid #f59e0b;
            }
            .diff-new .diff-content {
                border-left: 3px solid #10b981;
            }
            .diff-code pre {
                white-space: pre-wrap;
                word-break: break-word;
                font-family: 'JetBrains Mono', monospace;
                font-size: 0.8rem;
                margin: 0;
            }
            .color-row {
                display: flex;
                align-items: center;
                gap: 0.5rem;
                margin-bottom: 0.25rem;
            }
            .color-swatch {
                display: inline-block;
                width: 20px;
                height: 20px;
                border-radius: 4px;
                border: 1px solid rgba(255,255,255,0.2);
            }
            @media (max-width: 768px) {
                .diff-row {
                    grid-template-columns: 1fr;
                }
            }
        </style>

        <script>
        function confirmRevert(form, version) {
            event.preventDefault();

            var overlay = document.createElement('div');
            overlay.className = 'modal-overlay';
            overlay.innerHTML = '<div class="modal-box">' +
                '<h3>Revert Theme to Version ' + version + '?</h3>' +
                '<p style="color: var(--text-muted); margin-bottom: 1.5rem;">This will restore the theme settings from version ' + version + '. A new version will be created with the restored settings.</p>' +
                '<div class="modal-actions">' +
                '<button type="button" class="btn btn-outline" onclick="this.closest(\'.modal-overlay\').remove()">Cancel</button>' +
                '<button type="button" class="btn btn-primary" id="confirmRevertBtn">Revert Theme</button>' +
                '</div></div>';
            document.body.appendChild(overlay);

            document.getElementById('confirmRevertBtn').addEventListener('click', function() {
                form.submit();
            });

            overlay.addEventListener('click', function(e) {
                if (e.target === overlay) overlay.remove();
            });

            return false;
        }

        document.addEventListener('DOMContentLoaded', function() {
            // Process each diff section
            document.querySelectorAll('.diff-section').forEach(function(section) {
                var oldEl = section.querySelector('[data-old]');
                var newEl = section.querySelector('[data-new]');
                var badge = section.querySelector('.diff-badge');

                if (!oldEl || !newEl || !badge) return;

                var oldVal = oldEl.getAttribute('data-old') || '';
                var newVal = newEl.getAttribute('data-new') || '';

                if (oldVal === newVal) {
                    badge.textContent = 'Unchanged';
                    badge.className = 'diff-badge unchanged';
                    section.classList.add('no-changes');
                } else {
                    badge.textContent = 'Changed';
                    badge.className = 'diff-badge changed';
                    section.classList.add('has-changes');
                }
            });
        });
        </script>
    ` + adminLayoutEnd,

	"folders_list": adminLayoutStart + `
        <div class="page-header">
            <h1>Folders</h1>
            <a href="/cm/folders/new" class="btn btn-primary">New Folder</a>
        </div>
        <p class="page-subtitle">Organize your content into folders to create clean URL structures like /blog/2024/post-name</p>
        {{if not .Folders}}
        <div class="empty-state">
            <p>No folders yet. Create your first folder to start organizing content.</p>
        </div>
        {{else}}
        <div class="table-container">
            <table>
                <thead>
                    <tr>
                        <th>Folder</th>
                        <th>Path</th>
                        <th>Actions</th>
                    </tr>
                </thead>
                <tbody>
                    {{range .FolderTree}}
                    <tr>
                        <td style="padding-left: {{if .Depth}}{{multiply .Depth 24}}px{{else}}0{{end}}">
                            <span style="color: var(--accent);">📁</span>
                            <strong>{{.Folder.Name}}</strong>
                        </td>
                        <td><code>{{.Folder.Path}}</code></td>
                        <td class="actions">
                            <a href="/cm/folders/{{.Folder.ID.Hex}}" class="btn btn-sm">Edit</a>
                            <form method="POST" action="/cm/folders/{{.Folder.ID.Hex}}/delete" onsubmit="return confirmDelete(this, 'Are you sure you want to delete this folder?<br><br><span style=&quot;color: var(--text-muted); font-size: 0.9rem;&quot;>Make sure it has no content or subfolders.</span>')">
                                {{$.CSRFField}}
                                <button type="submit" class="btn btn-sm btn-danger">Delete</button>
                            </form>
                        </td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>
        {{end}}
        <style>
            .empty-state {
                background: var(--bg-card);
                border: 1px dashed var(--border);
                border-radius: var(--radius);
                padding: 3rem;
                text-align: center;
                color: var(--text-muted);
            }
            code {
                font-family: 'JetBrains Mono', monospace;
                background: rgba(99, 102, 241, 0.1);
                padding: 0.25rem 0.5rem;
                border-radius: 4px;
                font-size: 0.85rem;
            }
        </style>
    ` + adminLayoutEnd,

	"folder_form": adminLayoutStart + `
        <div class="page-header">
            <h1>{{if .IsNew}}New Folder{{else}}Edit Folder{{end}}</h1>
        </div>
        {{if .Error}}<div class="error-message">{{.Error}}</div>{{end}}
        <form method="POST" class="form-card">
            {{.CSRFField}}
            <div class="form-group">
                <label for="name">Folder Name</label>
                <input type="text" id="name" name="name" value="{{if .Folder}}{{.Folder.Name}}{{end}}" required placeholder="e.g., Blog Posts">
            </div>
            <div class="form-group">
                <label for="slug">URL Segment (slug)</label>
                <input type="text" id="slug" name="slug" value="{{if .Folder}}{{.Folder.Slug}}{{end}}" placeholder="auto-generated from name">
                <p class="help-text">This will be part of the URL path. Leave empty to auto-generate.</p>
            </div>
            <div class="form-group">
                <label for="parent_id">Parent Folder</label>
                <select id="parent_id" name="parent_id">
                    <option value="root">/ (root)</option>
                    {{range .Folders}}
                    {{if $.Folder}}
                    {{if ne .ID.Hex $.Folder.ID.Hex}}
                    <option value="{{.ID.Hex}}" {{if $.Folder.ParentID}}{{if eq .ID.Hex $.Folder.ParentID.Hex}}selected{{end}}{{end}}>{{.Path}}</option>
                    {{end}}
                    {{else}}
                    <option value="{{.ID.Hex}}">{{.Path}}</option>
                    {{end}}
                    {{end}}
                </select>
            </div>

            {{if .Folder}}
            <div class="form-section">
                <h3>Current Path</h3>
                <p><code>{{.Folder.Path}}</code></p>
            </div>
            {{end}}

            <div class="form-actions">
                <a href="/cm/folders" class="btn btn-outline">Cancel</a>
                <button type="submit" class="btn btn-primary">{{if .IsNew}}Create Folder{{else}}Update Folder{{end}}</button>
            </div>
        </form>
        <style>
            code {
                font-family: 'JetBrains Mono', monospace;
                background: rgba(99, 102, 241, 0.1);
                padding: 0.25rem 0.5rem;
                border-radius: 4px;
                font-size: 0.9rem;
            }
        </style>
    ` + adminLayoutEnd,

	"security": adminLayoutStart + `
        <div class="page-header">
            <h1>Security Settings</h1>
        </div>
        {{if .IsDefaultPassword}}
        <div class="security-alert" style="margin-bottom: 2rem;">
            <div class="alert-icon">⚠️</div>
            <div class="alert-content">
                <strong>Warning:</strong> You are using the default password. Please change it immediately to secure your site.
            </div>
        </div>
        {{end}}
        {{if .Error}}<div class="error-message">{{.Error}}</div>{{end}}
        {{if .Success}}<div class="success-message">{{.Success}}</div>{{end}}
        <form method="POST" class="form-card">
            {{.CSRFField}}
            <div class="form-section">
                <h3>Change Password</h3>
                <p class="help-text">Password must be at least 8 characters and contain uppercase, lowercase, and numbers.</p>
                <div class="form-group">
                    <label for="current_password">Current Password</label>
                    <input type="password" id="current_password" name="current_password" required autocomplete="current-password">
                </div>
                <div class="form-group">
                    <label for="new_password">New Password</label>
                    <input type="password" id="new_password" name="new_password" required autocomplete="new-password">
                </div>
                <div class="form-group">
                    <label for="confirm_password">Confirm New Password</label>
                    <input type="password" id="confirm_password" name="confirm_password" required autocomplete="new-password">
                </div>
            </div>

            <div class="form-section">
                <h3>Password Requirements</h3>
                <ul class="password-requirements">
                    <li>At least 8 characters long</li>
                    <li>At least one uppercase letter (A-Z)</li>
                    <li>At least one lowercase letter (a-z)</li>
                    <li>At least one number (0-9)</li>
                </ul>
            </div>

            <div class="form-actions">
                <button type="submit" class="btn btn-primary">Change Password</button>
            </div>
        </form>
    ` + adminLayoutEnd,

	"config": adminLayoutStart + `
        <div class="page-header">
            <h1>Configuration</h1>
        </div>
        {{if .Error}}<div class="error-message">{{.Error}}</div>{{end}}
        {{if .Success}}<div class="success-message">{{.Success}}</div>{{end}}

        <div class="form-card" style="margin-bottom: 1.5rem;">
            <div class="form-section">
                <h3>Version Information</h3>
                <div class="version-info">
                    <div class="version-row">
                        <span class="version-label">Software Version:</span>
                        <span class="version-value">{{.SoftwareVersion}} ({{.EnvLabel}})</span>
                    </div>
                    <div class="version-row">
                        <span class="version-label">Database Version:</span>
                        <span class="version-value">{{if .DatabaseVersion}}{{.DatabaseVersion}}{{else}}<em>Not set</em>{{end}}</span>
                    </div>
                </div>
            </div>
        </div>
        <style>
            .version-info { display: flex; flex-direction: column; gap: 0.5rem; }
            .version-row { display: flex; align-items: center; gap: 1rem; }
            .version-label { font-weight: 500; color: var(--text-muted); min-width: 150px; }
            .version-value { font-family: monospace; background: var(--bg-dark); padding: 0.25rem 0.75rem; border-radius: 4px; }
        </style>

        <form method="POST" class="form-card">
            {{.CSRFField}}
            <div class="form-section">
                <h3>Page Title Templates</h3>
                <p class="help-text">Customize how page titles appear in browser tabs and search results.</p>
                <p class="help-text">Available variables: <code>{{"{{"}}title{{"}}"}}</code> (page title), <code>{{"{{"}}site_name{{"}}"}}</code> (from Theme settings: "{{.SiteName}}")</p>

                <div class="form-group">
                    <label for="title_template">Title Template (when page has a title)</label>
                    <input type="text" id="title_template" name="title_template" value="{{.Config.TitleTemplate}}" placeholder="{{"{{"}}title{{"}}"}} - {{"{{"}}site_name{{"}}"}}">
                    <p class="help-text">Example result: "About Us - {{.SiteName}}"</p>
                </div>

                <div class="form-group">
                    <label for="title_template_no_title">Title Template (when page has no title)</label>
                    <input type="text" id="title_template_no_title" name="title_template_no_title" value="{{.Config.TitleTemplateNoTitle}}" placeholder="{{"{{"}}site_name{{"}}"}}">
                    <p class="help-text">Example result: "{{.SiteName}}"</p>
                </div>
            </div>

            <div class="form-actions">
                <button type="submit" class="btn btn-primary">Save Configuration</button>
            </div>
        </form>
    ` + adminLayoutEnd,

	"redirects_list": adminLayoutStart + `
        <div class="page-header">
            <h1>Redirects</h1>
            <a href="/cm/redirects/new" class="btn btn-primary">New Redirect</a>
        </div>
        <div class="table-container">
            <table>
                <thead>
                    <tr>
                        <th>From</th>
                        <th>To</th>
                        <th>Status</th>
                        <th>Description</th>
                        <th>Actions</th>
                    </tr>
                </thead>
                <tbody>
                    {{range .Redirects}}
                    <tr>
                        <td><code>{{.FromPath}}</code></td>
                        <td><code>{{.ToPath}}</code></td>
                        <td><span class="status-badge">{{.StatusCode}}</span></td>
                        <td>{{.Description}}</td>
                        <td class="actions">
                            <a href="/cm/redirects/{{.ID.Hex}}" class="btn btn-sm">Edit</a>
                            <form method="POST" action="/cm/redirects/{{.ID.Hex}}/delete" style="display:inline" onsubmit="return confirmDelete(this, 'Are you sure you want to delete this redirect?&lt;br&gt;&lt;br&gt;Note: Browsers cache 301 redirects. After deletion, users may need to clear their browser cache.')">
            {{$.CSRFField}}
                                <button type="submit" class="btn btn-sm btn-danger">Delete</button>
                            </form>
                        </td>
                    </tr>
                    {{else}}
                    <tr>
                        <td colspan="5" class="text-muted" style="text-align:center;padding:2rem;">No redirects configured yet.</td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>
    ` + adminLayoutEnd,

	"redirect_form": adminLayoutStart + `
        <div class="page-header">
            <h1>{{if .IsNew}}New Redirect{{else}}Edit Redirect{{end}}</h1>
        </div>
        {{if .Error}}<div class="error-message">{{.Error}}</div>{{end}}
        <form method="POST" action="{{if .IsNew}}/cm/redirects/new{{else}}/cm/redirects/{{.Redirect.ID.Hex}}{{end}}" class="form-card">
            {{.CSRFField}}
            <div class="form-group">
                <label for="from_path">From Path</label>
                <input type="text" id="from_path" name="from_path" value="{{if .Redirect}}{{.Redirect.FromPath}}{{end}}" placeholder="/old-page" required>
                <p class="help-text">The path to redirect from (e.g., /machine-intelligence)</p>
            </div>
            <div class="form-group">
                <label for="to_path">To Path</label>
                <input type="text" id="to_path" name="to_path" value="{{if .Redirect}}{{.Redirect.ToPath}}{{end}}" placeholder="/new-page" required>
                <p class="help-text">The destination path (e.g., /artificial-intelligence) or full URL</p>
            </div>
            <div class="form-group">
                <label for="status_code">Redirect Type</label>
                <select id="status_code" name="status_code">
                    <option value="301" {{if .Redirect}}{{if eq .Redirect.StatusCode 301}}selected{{end}}{{end}}>301 (Permanent)</option>
                    <option value="302" {{if .Redirect}}{{if eq .Redirect.StatusCode 302}}selected{{end}}{{end}}>302 (Temporary)</option>
                </select>
                <p class="help-text">Use 301 for permanent redirects (SEO-friendly), 302 for temporary</p>
            </div>
            <div class="form-group">
                <label for="description">Description (optional)</label>
                <input type="text" id="description" name="description" value="{{if .Redirect}}{{.Redirect.Description}}{{end}}" placeholder="Why this redirect exists">
            </div>
            <div class="form-actions">
                <a href="/cm/redirects" class="btn btn-outline">Cancel</a>
                <button type="submit" class="btn btn-primary">{{if .IsNew}}Create{{else}}Update{{end}}</button>
            </div>
        </form>
    ` + adminLayoutEnd,

	"contact_messages_list": adminLayoutStart + `
        <div class="page-header">
            <h1>Messages {{if .UnreadCount}}<span class="badge">{{.UnreadCount}} unread</span>{{end}}</h1>
        </div>
        <style>
            .badge { background: var(--primary); color: white; padding: 0.25rem 0.5rem; border-radius: 12px; font-size: 0.8rem; margin-left: 0.5rem; }
            .unread { background: rgba(99, 102, 241, 0.1); }
            .message-preview { max-width: 300px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--text-muted); }
        </style>
        <div class="table-card">
            <table class="data-table">
                <thead>
                    <tr>
                        <th>Date</th>
                        <th>Name</th>
                        <th>Email</th>
                        <th>Message</th>
                        <th>Actions</th>
                    </tr>
                </thead>
                <tbody>
                    {{range .Messages}}
                    <tr {{if not .Read}}class="unread"{{end}}>
                        <td>{{.CreatedAt.Format "Jan 2, 2006 3:04 PM"}}</td>
                        <td>{{if not .Read}}<strong>{{.Name}}</strong>{{else}}{{.Name}}{{end}}</td>
                        <td>{{if and .Email (not .IsSystem)}}<a href="mailto:{{.Email}}">{{.Email}}</a>{{else if .Email}}{{.Email}}{{else}}<em>N/A</em>{{end}}</td>
                        <td class="message-preview">{{.Message}}</td>
                        <td class="actions">
                            <a href="/cm/messages/{{.ID.Hex}}" class="btn btn-sm">View</a>
                            <form method="POST" action="/cm/messages/{{.ID.Hex}}/delete" style="display:inline" onsubmit="return confirmDelete(this, 'Are you sure you want to delete this message?')">
            {{$.CSRFField}}
                                <button type="submit" class="btn btn-sm btn-danger">Delete</button>
                            </form>
                        </td>
                    </tr>
                    {{else}}
                    <tr>
                        <td colspan="5" class="text-muted" style="text-align:center;padding:2rem;">No messages yet.</td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>
    ` + adminLayoutEnd,

	"contact_message_view": adminLayoutStart + `
        <div class="page-header">
            <h1>{{if .Message.Subject}}{{.Message.Subject}}{{else}}Message from {{.Message.Name}}{{end}}</h1>
            <a href="/cm/messages" class="btn btn-outline">Back to Messages</a>
        </div>
        <style>
            .message-card { background: var(--bg-card); border: 1px solid var(--border); border-radius: var(--radius); padding: 2rem; margin-bottom: 1rem; }
            .message-meta { display: grid; grid-template-columns: repeat(2, 1fr); gap: 1rem; margin-bottom: 1.5rem; padding-bottom: 1.5rem; border-bottom: 1px solid var(--border); }
            .message-meta-item label { display: block; font-size: 0.8rem; color: var(--text-muted); margin-bottom: 0.25rem; }
            .message-meta-item span { font-size: 1rem; }
            .message-body { white-space: pre-wrap; line-height: 1.6; }
            .message-body-html { line-height: 1.6; }
            .message-body-html a { color: var(--primary); }
            .message-footer { margin-top: 1.5rem; padding-top: 1.5rem; border-top: 1px solid var(--border); font-size: 0.85rem; color: var(--text-muted); }
            .system-badge { display: inline-block; background: var(--primary); color: white; font-size: 0.7rem; padding: 0.15rem 0.5rem; border-radius: 4px; margin-left: 0.5rem; }
        </style>
        <div class="message-card">
            <div class="message-meta">
                <div class="message-meta-item">
                    <label>From</label>
                    <span>{{.Message.Name}}{{if .Message.IsSystem}}<span class="system-badge">System</span>{{end}}</span>
                </div>
                {{if not .Message.IsSystem}}
                <div class="message-meta-item">
                    <label>Email</label>
                    <span>{{if .Message.Email}}<a href="mailto:{{.Message.Email}}">{{.Message.Email}}</a>{{else}}<em>N/A</em>{{end}}</span>
                </div>
                {{end}}
                <div class="message-meta-item">
                    <label>Received</label>
                    <span>{{.Message.CreatedAt.Format "January 2, 2006 at 3:04 PM"}}</span>
                </div>
                {{if not .Message.IsSystem}}
                <div class="message-meta-item">
                    <label>IP Address</label>
                    <span>{{.Message.IPAddress}}</span>
                </div>
                {{end}}
            </div>
            {{if .Message.IsSystem}}
            <div class="message-body-html">{{safeHTML .Message.Message}}</div>
            {{else}}
            <div class="message-body">{{.Message.Message}}</div>
            {{end}}
            {{if not .Message.IsSystem}}
            <div class="message-footer">
                User Agent: {{.Message.UserAgent}}
            </div>
            {{end}}
        </div>
        <div class="form-actions">
            {{if not .Message.IsSystem}}
            <a href="mailto:{{.Message.Email}}?subject=Re: Your message" class="btn btn-primary">Reply via Email</a>
            {{end}}
            <form method="POST" action="/cm/messages/{{.Message.ID.Hex}}/delete" style="display:inline" onsubmit="return confirmDelete(this, 'Are you sure you want to delete this message?')">
            {{.CSRFField}}
                <button type="submit" class="btn btn-danger">Delete</button>
            </form>
        </div>
    ` + adminLayoutEnd,

	"asset_library": adminLayoutStart + `
        <div class="page-header">
            <h1>Asset Library</h1>
            <a href="/cm/assets/upload" class="btn btn-primary">Upload Asset</a>
        </div>

        <div class="filter-bar" style="margin-bottom: 1rem;">
            <label style="margin-right: 0.5rem;">Filter by folder:</label>
            <select onchange="window.location.href='/cm/assets?folder=' + this.value" style="padding: 0.5rem; background: var(--bg-card); border: 1px solid rgba(255,255,255,0.1); border-radius: 6px; color: var(--text);">
                <option value="">All folders</option>
                {{range .Folders}}
                <option value="{{.}}" {{if eq . $.CurrentFolder}}selected{{end}}>{{.}}</option>
                {{end}}
            </select>
        </div>

        {{if .Assets}}
        <div class="table-container">
            <table>
                <thead>
                    <tr>
                        <th>Preview</th>
                        <th>Filename</th>
                        <th>Serve Path</th>
                        <th>Type</th>
                        <th>Size</th>
                        <th>Actions</th>
                    </tr>
                </thead>
                <tbody>
                    {{range .Assets}}
                    <tr>
                        <td style="width: 60px;">
                            {{if or (eq .MimeType "image/png") (eq .MimeType "image/jpeg") (eq .MimeType "image/gif") (eq .MimeType "image/webp") (eq .MimeType "image/svg+xml")}}
                            <img src="{{.ServePath}}" alt="{{.Filename}}" style="max-width: 50px; max-height: 50px; border-radius: 4px;">
                            {{else}}
                            <span style="font-size: 1.5rem;">📄</span>
                            {{end}}
                        </td>
                        <td>{{.Filename}}</td>
                        <td><code>{{.ServePath}}</code></td>
                        <td>{{.MimeType}}</td>
                        <td>{{if lt .Size 1024}}{{.Size}} B{{else if lt .Size 1048576}}{{divide .Size 1024}} KB{{else}}{{divide .Size 1048576}} MB{{end}}</td>
                        <td>
                            <a href="{{.ServePath}}" target="_blank" class="btn btn-sm">View</a>
                            <button onclick="copyToClipboard('{{.ServePath}}')" class="btn btn-sm btn-secondary">Copy URL</button>
                            <form method="POST" action="/cm/assets/{{.ID.Hex}}/delete" style="display:inline" onsubmit="return confirmDelete(this, 'Are you sure you want to delete this asset?')">
            {{$.CSRFField}}
                                <button type="submit" class="btn btn-sm btn-danger">Delete</button>
                            </form>
                        </td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>
        {{else}}
        <div class="empty-state">
            <p>No assets yet. <a href="/cm/assets/upload">Upload your first asset</a></p>
        </div>
        {{end}}

        <script>
        function copyToClipboard(text) {
            navigator.clipboard.writeText(window.location.origin + text).then(function() {
                alert('URL copied to clipboard!');
            });
        }
        </script>
    ` + adminLayoutEnd,

	"broken_links": adminLayoutStart + `
        <div class="page-header">
            <h1>🔗 Broken Link Finder</h1>
        </div>

        <div class="info-card" style="margin-bottom: 1.5rem;">
            <p style="margin: 0; color: var(--text-muted);">
                This tool scans all published content for broken links, including both internal pages and external URLs.
                Links with redirects are considered valid.
            </p>
        </div>

        <!-- Progress Section -->
        <div id="scan-progress" style="margin-bottom: 1.5rem;">
            <div style="background: var(--bg-card); padding: 1.5rem; border-radius: 8px;">
                <div style="display: flex; align-items: center; gap: 1rem; margin-bottom: 1rem;">
                    <div class="spinner" style="width: 24px; height: 24px; border: 3px solid var(--bg-dark); border-top-color: var(--primary); border-radius: 50%; animation: spin 1s linear infinite;"></div>
                    <div style="flex: 1; min-width: 0;">
                        <div id="progress-text" style="font-weight: 500;">Starting scan...</div>
                        <div id="progress-path" style="font-size: 0.875rem; color: var(--text-muted); white-space: nowrap; overflow: hidden; text-overflow: ellipsis;"></div>
                        <div id="progress-checking" style="font-size: 0.75rem; color: var(--primary); margin-top: 0.25rem; white-space: nowrap; overflow: hidden; text-overflow: ellipsis;"></div>
                    </div>
                </div>
                <div style="background: var(--bg-dark); height: 8px; border-radius: 4px; overflow: hidden;">
                    <div id="progress-bar" style="background: var(--primary); height: 100%; width: 0%; transition: width 0.2s;"></div>
                </div>
                <div id="progress-links" style="font-size: 0.75rem; color: var(--text-muted); margin-top: 0.5rem;">Links checked: 0</div>
            </div>
        </div>

        <!-- Fix Link Modal -->
        <div id="fix-modal" style="display: none; position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.7); z-index: 1000; align-items: center; justify-content: center;">
            <div style="background: var(--bg-card); border-radius: 12px; padding: 1.5rem; max-width: 600px; width: 90%; max-height: 80vh; overflow-y: auto;">
                <h3 style="margin: 0 0 1rem 0;">Fix Broken Link</h3>
                <div style="margin-bottom: 1rem;">
                    <label style="display: block; margin-bottom: 0.5rem; color: var(--text-muted); font-size: 0.875rem;">Page</label>
                    <div id="fix-page-title" style="font-weight: 500;"></div>
                </div>
                <div style="margin-bottom: 1rem;">
                    <label style="display: block; margin-bottom: 0.5rem; color: var(--text-muted); font-size: 0.875rem;">Field</label>
                    <div id="fix-field-name" style="font-weight: 500;"></div>
                </div>
                <div style="margin-bottom: 1rem;">
                    <label style="display: block; margin-bottom: 0.5rem; color: var(--text-muted); font-size: 0.875rem;">Current Link (broken)</label>
                    <code id="fix-old-url" style="display: block; background: rgba(255,107,107,0.2); color: var(--danger); padding: 0.5rem; border-radius: 4px; word-break: break-all;"></code>
                </div>
                <div style="margin-bottom: 1.5rem;">
                    <label for="fix-new-url" style="display: block; margin-bottom: 0.5rem; color: var(--text-muted); font-size: 0.875rem;">New Link</label>
                    <input type="text" id="fix-new-url" style="width: 100%; padding: 0.75rem; background: var(--bg-dark); border: 1px solid rgba(255,255,255,0.1); border-radius: 8px; color: var(--text); font-size: 1rem;" placeholder="Enter the corrected URL">
                    <small style="color: var(--text-muted); display: block; margin-top: 0.25rem;">Leave empty to remove the link entirely</small>
                </div>
                <div id="fix-status" style="display: none; margin-bottom: 1rem; padding: 0.75rem; border-radius: 6px;"></div>
                <div style="display: flex; gap: 0.5rem; justify-content: flex-end;">
                    <button id="fix-cancel" class="btn btn-secondary">Cancel</button>
                    <button id="fix-save" class="btn btn-primary">Save Fix</button>
                </div>
            </div>
        </div>

        <style>
            @keyframes spin {
                to { transform: rotate(360deg); }
            }
        </style>

        <div class="stats-row" style="display: flex; gap: 1rem; margin-bottom: 1.5rem;">
            <div class="stat-card" style="background: var(--bg-card); padding: 1rem 1.5rem; border-radius: 8px; flex: 1;">
                <div id="stat-scanned" style="font-size: 2rem; font-weight: bold; color: var(--primary);">0</div>
                <div style="color: var(--text-muted); font-size: 0.875rem;">Pages Scanned</div>
            </div>
            <div class="stat-card" style="background: var(--bg-card); padding: 1rem 1.5rem; border-radius: 8px; flex: 1;">
                <div id="stat-links" style="font-size: 2rem; font-weight: bold; color: var(--primary);">0</div>
                <div style="color: var(--text-muted); font-size: 0.875rem;">Links Checked</div>
            </div>
            <div class="stat-card" style="background: var(--bg-card); padding: 1rem 1.5rem; border-radius: 8px; flex: 1;">
                <div id="stat-pages-broken" style="font-size: 2rem; font-weight: bold; color: var(--success);">0</div>
                <div style="color: var(--text-muted); font-size: 0.875rem;">Pages with Issues</div>
            </div>
            <div class="stat-card" style="background: var(--bg-card); padding: 1rem 1.5rem; border-radius: 8px; flex: 1;">
                <div id="stat-total-broken" style="font-size: 2rem; font-weight: bold; color: var(--success);">0</div>
                <div style="color: var(--text-muted); font-size: 0.875rem;">Broken Links</div>
            </div>
        </div>

        <div id="results-container">
            <div class="table-container" id="results-table" style="display: none;">
                <table class="data-table">
                    <thead>
                        <tr>
                            <th>Page</th>
                            <th>Broken Links</th>
                            <th>Actions</th>
                        </tr>
                    </thead>
                    <tbody id="results-body">
                    </tbody>
                </table>
            </div>
        </div>

        <div id="no-results" class="empty-state" style="display: none; background: rgba(72, 187, 120, 0.1); border: 1px solid rgba(72, 187, 120, 0.3);">
            <p style="color: var(--success); font-size: 1.25rem; margin: 0;">✓ No broken links found!</p>
            <p style="color: var(--text-muted); margin-top: 0.5rem;">All links in your published content are valid.</p>
        </div>

        <script>
        (function() {
            var pagesWithBrokenLinks = 0;
            var totalBrokenLinks = 0;
            var currentFix = null;

            function escapeHtml(text) {
                var div = document.createElement('div');
                div.textContent = text;
                return div.innerHTML;
            }

            function formatLinkError(link) {
                var details = '';
                if (link.status) {
                    details = ' <span style="background: rgba(255,107,107,0.3); padding: 0.1rem 0.3rem; border-radius: 3px; font-size: 0.7rem;">HTTP ' + link.status + '</span>';
                } else if (link.error) {
                    var shortError = link.error;
                    if (shortError.length > 40) shortError = shortError.substring(0, 40) + '...';
                    details = ' <span style="background: rgba(255,107,107,0.3); padding: 0.1rem 0.3rem; border-radius: 3px; font-size: 0.7rem;" title="' + escapeHtml(link.error) + '">' + escapeHtml(shortError) + '</span>';
                }
                return details;
            }

            function openFixModal(contentId, title, field, oldUrl, linkElement) {
                currentFix = { contentId: contentId, field: field, oldUrl: oldUrl, linkElement: linkElement };
                document.getElementById('fix-page-title').textContent = title;
                document.getElementById('fix-field-name').textContent = field;
                document.getElementById('fix-old-url').textContent = oldUrl;
                document.getElementById('fix-new-url').value = oldUrl;
                document.getElementById('fix-status').style.display = 'none';
                document.getElementById('fix-modal').style.display = 'flex';
                document.getElementById('fix-new-url').focus();
                document.getElementById('fix-new-url').select();
            }

            function closeFixModal() {
                document.getElementById('fix-modal').style.display = 'none';
                currentFix = null;
            }

            function showFixStatus(message, isError) {
                var status = document.getElementById('fix-status');
                status.textContent = message;
                status.style.display = 'block';
                status.style.background = isError ? 'rgba(255,107,107,0.2)' : 'rgba(72,187,120,0.2)';
                status.style.color = isError ? 'var(--danger)' : 'var(--success)';
            }

            function saveFix() {
                if (!currentFix) return;

                var newUrl = document.getElementById('fix-new-url').value.trim();
                var saveBtn = document.getElementById('fix-save');
                saveBtn.disabled = true;
                saveBtn.textContent = 'Saving...';

                fetch('/api/tools/fix-link', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        contentId: currentFix.contentId,
                        field: currentFix.field,
                        oldUrl: currentFix.oldUrl,
                        newUrl: newUrl
                    })
                })
                .then(function(resp) {
                    return resp.json().then(function(data) {
                        if (!resp.ok) throw new Error(data.error || 'Failed to save');
                        return data;
                    });
                })
                .then(function(data) {
                    showFixStatus('Link fixed successfully! Version ' + data.version + ' created.', false);
                    // Update the UI to show the link is fixed
                    if (currentFix.linkElement) {
                        currentFix.linkElement.style.background = 'rgba(72,187,120,0.2)';
                        currentFix.linkElement.style.color = 'var(--success)';
                        currentFix.linkElement.textContent = newUrl || '(removed)';
                        // Remove the fix button
                        var fixBtn = currentFix.linkElement.parentElement.querySelector('.fix-btn');
                        if (fixBtn) fixBtn.remove();
                    }
                    setTimeout(closeFixModal, 1500);
                })
                .catch(function(err) {
                    showFixStatus('Error: ' + err.message, true);
                })
                .finally(function() {
                    saveBtn.disabled = false;
                    saveBtn.textContent = 'Save Fix';
                });
            }

            // Set up modal event listeners
            document.getElementById('fix-cancel').addEventListener('click', closeFixModal);
            document.getElementById('fix-save').addEventListener('click', saveFix);
            document.getElementById('fix-modal').addEventListener('click', function(e) {
                if (e.target === this) closeFixModal();
            });
            document.getElementById('fix-new-url').addEventListener('keydown', function(e) {
                if (e.key === 'Enter') saveFix();
                if (e.key === 'Escape') closeFixModal();
            });

            function addResult(result) {
                var table = document.getElementById('results-table');
                var tbody = document.getElementById('results-body');
                table.style.display = 'block';

                pagesWithBrokenLinks++;
                totalBrokenLinks += result.brokenLinks.length;

                // Update stats with danger color
                var statPagesBroken = document.getElementById('stat-pages-broken');
                var statTotalBroken = document.getElementById('stat-total-broken');
                statPagesBroken.textContent = pagesWithBrokenLinks;
                statPagesBroken.style.color = 'var(--danger)';
                statTotalBroken.textContent = totalBrokenLinks;
                statTotalBroken.style.color = 'var(--danger)';

                var linksHtml = result.brokenLinks.map(function(link, idx) {
                    var isExternal = link.url.startsWith('http') || link.url.startsWith('//');
                    var icon = isExternal ? '🌐' : '📄';
                    var linkId = 'link-' + result.id + '-' + idx;
                    return '<div style="margin-bottom: 0.5rem;">' +
                        '<span style="margin-right: 0.25rem;">' + icon + '</span>' +
                        '<code id="' + linkId + '" style="background: rgba(255,107,107,0.2); color: var(--danger); padding: 0.25rem 0.5rem; border-radius: 4px; word-break: break-all;">' +
                        escapeHtml(link.url) + '</code>' +
                        formatLinkError(link) +
                        '<span style="color: var(--text-muted); font-size: 0.75rem; margin-left: 0.5rem;">in: ' + escapeHtml(link.field) + '</span>' +
                        '</div>';
                }).join('');

                // Build fix buttons for each broken link
                var fixButtonsHtml = result.brokenLinks.map(function(link, idx) {
                    var linkId = 'link-' + result.id + '-' + idx;
                    return '<button class="btn btn-small fix-btn" data-content-id="' + escapeHtml(result.id) + '" data-title="' + escapeHtml(result.title) + '" data-field="' + escapeHtml(link.field) + '" data-url="' + escapeHtml(link.url) + '" data-link-id="' + linkId + '" style="margin-bottom: 0.25rem;">Fix</button>';
                }).join(' ');

                var row = document.createElement('tr');
                row.innerHTML = '<td><strong>' + escapeHtml(result.title) + '</strong>' +
                    '<div style="font-size: 0.875rem; color: var(--text-muted);">' + escapeHtml(result.path) + '</div></td>' +
                    '<td>' + linksHtml + '</td>' +
                    '<td style="white-space: nowrap;">' +
                    '<a href="' + escapeHtml(result.path) + '" target="_blank" class="btn btn-small">View</a> ' +
                    '<a href="/cm/content/' + escapeHtml(result.id) + '" class="btn btn-small btn-secondary">Edit</a> ' +
                    fixButtonsHtml + '</td>';

                // Add click handlers for fix buttons
                row.querySelectorAll('.fix-btn').forEach(function(btn) {
                    btn.addEventListener('click', function() {
                        var linkElement = document.getElementById(this.dataset.linkId);
                        openFixModal(this.dataset.contentId, this.dataset.title, this.dataset.field, this.dataset.url, linkElement);
                    });
                });

                tbody.appendChild(row);
            }

            function startScan() {
                var eventSource = new EventSource('/api/tools/broken-links/scan');

                eventSource.addEventListener('total', function(e) {
                    var data = JSON.parse(e.data);
                    document.getElementById('progress-text').textContent = 'Scanning ' + data.total + ' pages...';
                });

                eventSource.addEventListener('progress', function(e) {
                    var data = JSON.parse(e.data);
                    var percent = Math.round((data.current / data.total) * 100);
                    document.getElementById('progress-bar').style.width = percent + '%';
                    document.getElementById('progress-text').textContent = 'Scanning page ' + data.current + ' of ' + data.total + '...';
                    document.getElementById('progress-path').textContent = data.title + ' (' + data.path + ')';
                    document.getElementById('stat-scanned').textContent = data.current;

                    if (data.linksChecked) {
                        document.getElementById('progress-links').textContent = 'Links checked: ' + data.linksChecked;
                        document.getElementById('stat-links').textContent = data.linksChecked;
                    }

                    if (data.checking) {
                        document.getElementById('progress-checking').textContent = 'Checking: ' + data.checking;
                    } else {
                        document.getElementById('progress-checking').textContent = '';
                    }
                });

                eventSource.addEventListener('result', function(e) {
                    var result = JSON.parse(e.data);
                    addResult(result);
                });

                eventSource.addEventListener('complete', function(e) {
                    var data = JSON.parse(e.data);
                    eventSource.close();

                    // Hide progress section
                    document.getElementById('scan-progress').style.display = 'none';

                    // Update final stats
                    document.getElementById('stat-scanned').textContent = data.totalPages;
                    document.getElementById('stat-links').textContent = data.totalLinksChecked || 0;

                    // Show "no results" message if no broken links
                    if (data.pagesWithBrokenLinks === 0) {
                        document.getElementById('no-results').style.display = 'block';
                    }
                });

                eventSource.addEventListener('error', function(e) {
                    try {
                        var data = JSON.parse(e.data);
                        document.getElementById('progress-text').textContent = 'Error: ' + data;
                    } catch(err) {
                        document.getElementById('progress-text').textContent = 'Connection error';
                    }
                    eventSource.close();
                });

                eventSource.onerror = function() {
                    eventSource.close();
                    document.getElementById('scan-progress').style.display = 'none';
                };
            }

            startScan();
        })();
        </script>
    ` + adminLayoutEnd,

	"asset_upload": adminLayoutStart + `
        <div class="page-header">
            <h1>Upload Asset</h1>
            <a href="/cm/assets" class="btn btn-secondary">Back to Library</a>
        </div>

        <form method="POST" action="/cm/assets/upload" enctype="multipart/form-data" class="form-card">
            {{.CSRFField}}
            <div class="form-group">
                <label for="file">File</label>
                <input type="file" id="file" name="file" required style="padding: 0.75rem; background: var(--bg-dark); border: 1px solid rgba(255,255,255,0.1); border-radius: 8px; color: var(--text);">
            </div>

            <div class="form-group">
                <label for="serve_path">Serve Path (URL where the file will be accessible)</label>
                <input type="text" id="serve_path" name="serve_path" placeholder="e.g., /favicon.png or /images/logo.png" required>
                <small style="color: var(--text-muted);">The full URL path where this file will be served. Use / for root level files like favicons.</small>
            </div>

            <div class="form-group">
                <label for="description">Description (optional)</label>
                <input type="text" id="description" name="description" placeholder="Brief description of the asset">
            </div>

            <div class="form-actions">
                <button type="submit" class="btn btn-primary">Upload Asset</button>
            </div>
        </form>

        <script>
        // Auto-suggest serve path from filename
        document.getElementById('file').addEventListener('change', function(e) {
            var servePathInput = document.getElementById('serve_path');
            if (servePathInput.value === '' && e.target.files.length > 0) {
                servePathInput.value = '/' + e.target.files[0].name;
            }
        });
        </script>
    ` + adminLayoutEnd,
}

const adminLayoutStart = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>LightCMS Admin</title>
    <link rel="icon" type="image/x-icon" href="/static/images/favicon.ico">
    <link rel="icon" type="image/png" sizes="16x16" href="/static/images/favicon-16x16.png">
    <link rel="icon" type="image/png" sizes="32x32" href="/static/images/favicon-32x32.png">
    <link rel="icon" type="image/png" sizes="48x48" href="/static/images/favicon-48x48.png">
    <link rel="apple-touch-icon" sizes="180x180" href="/static/images/apple-touch-icon.png">
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&family=Space+Grotesk:wght@400;500;600;700&family=JetBrains+Mono:wght@400;500&display=swap" rel="stylesheet">
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        :root {
            --primary: #6366f1;
            --secondary: #8b5cf6;
            --accent: #06b6d4;
            --bg-dark: #0f172a;
            --bg-card: #1e1b4b;
            --bg-hover: #2e2a5a;
            --text: #f1f5f9;
            --text-muted: #94a3b8;
            --border: rgba(99, 102, 241, 0.2);
            --success: #10b981;
            --danger: #ef4444;
            --radius: 12px;
        }
        body {
            font-family: 'Inter', system-ui, sans-serif;
            background: var(--bg-dark);
            color: var(--text);
            min-height: 100vh;
        }
        a { color: var(--accent); text-decoration: none; }
        a:hover { text-decoration: underline; }

        .admin-layout {
            display: grid;
            grid-template-columns: 260px 1fr;
            min-height: 100vh;
        }
        .sidebar {
            background: linear-gradient(180deg, var(--bg-card) 0%, var(--bg-dark) 100%);
            border-right: 1px solid var(--border);
            padding: 1.5rem;
            position: sticky;
            top: 0;
            height: 100vh;
            overflow-y: auto;
        }
        .sidebar-logo {
            margin-bottom: 2rem;
            display: block;
        }
        .sidebar-logo img {
            height: 48px;
            width: auto;
        }
        .nav-section {
            margin-bottom: 1.5rem;
        }
        .nav-section-title {
            font-size: 0.7rem;
            text-transform: uppercase;
            letter-spacing: 0.1em;
            color: var(--text-muted);
            margin-bottom: 0.5rem;
            padding-left: 0.75rem;
        }
        .nav-link {
            display: flex;
            align-items: center;
            gap: 0.75rem;
            padding: 0.75rem;
            border-radius: var(--radius);
            color: var(--text);
            transition: all 0.2s;
            text-decoration: none;
        }
        .nav-link:hover {
            background: var(--bg-hover);
            text-decoration: none;
        }
        .logout-btn {
            background: none;
            border: none;
            cursor: pointer;
            width: 100%;
            text-align: left;
            font: inherit;
        }
        .nav-link.active {
            background: linear-gradient(135deg, var(--primary), var(--secondary));
        }
        .nav-badge {
            background: #ef4444;
            color: white;
            font-size: 0.7rem;
            font-weight: 600;
            padding: 0.15rem 0.5rem;
            border-radius: 9999px;
            margin-left: auto;
        }

        .main-content {
            padding: 2rem;
            max-width: 1400px;
        }
        .page-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 2rem;
        }
        .page-header h1 {
            font-family: 'Space Grotesk', sans-serif;
            font-size: 2rem;
            font-weight: 600;
        }
        .page-subtitle {
            color: var(--text-muted);
            margin-bottom: 1.5rem;
        }

        .btn {
            display: inline-flex;
            align-items: center;
            gap: 0.5rem;
            padding: 0.625rem 1.25rem;
            border-radius: var(--radius);
            font-weight: 500;
            font-size: 0.9rem;
            cursor: pointer;
            border: none;
            transition: all 0.2s;
            text-decoration: none;
        }
        .btn:hover { text-decoration: none; transform: translateY(-1px); }
        .btn-primary {
            background: linear-gradient(135deg, var(--primary), var(--secondary));
            color: white;
        }
        .btn-secondary {
            background: var(--bg-hover);
            color: var(--text);
            border: 1px solid var(--border);
        }
        .btn-outline {
            background: transparent;
            color: var(--text);
            border: 1px solid var(--border);
        }
        .btn-danger {
            background: var(--danger);
            color: white;
        }
        .btn-sm {
            padding: 0.375rem 0.75rem;
            font-size: 0.8rem;
        }
        .toggle-html-btn {
            font-family: monospace;
            font-size: 0.75rem;
        }
        .toggle-html-btn.active {
            background: var(--primary);
            color: white;
            border-color: var(--primary);
        }

        .table-container {
            background: var(--bg-card);
            border-radius: var(--radius);
            border: 1px solid var(--border);
            overflow: hidden;
        }
        table {
            width: 100%;
            border-collapse: collapse;
        }
        th, td {
            padding: 1rem;
            text-align: left;
            border-bottom: 1px solid var(--border);
        }
        th {
            background: rgba(99, 102, 241, 0.1);
            font-weight: 600;
            font-size: 0.85rem;
            text-transform: uppercase;
            letter-spacing: 0.05em;
        }
        tr:hover td {
            background: var(--bg-hover);
        }
        .actions {
            display: flex;
            gap: 0.5rem;
            align-items: center;
        }
        .actions form { display: inline; }

        .status-badge {
            display: inline-block;
            padding: 0.25rem 0.75rem;
            border-radius: 9999px;
            font-size: 0.75rem;
            font-weight: 500;
        }
        .status-badge.published {
            background: rgba(16, 185, 129, 0.2);
            color: var(--success);
        }
        .status-badge.draft {
            background: rgba(148, 163, 184, 0.2);
            color: var(--text-muted);
        }

        .form-card {
            background: var(--bg-card);
            border-radius: var(--radius);
            border: 1px solid var(--border);
            padding: 2rem;
        }
        .form-section {
            margin-bottom: 2rem;
            padding-bottom: 2rem;
            border-bottom: 1px solid var(--border);
        }
        .form-section h3 {
            margin-bottom: 1rem;
            font-size: 1.1rem;
        }
        .form-group {
            margin-bottom: 1.25rem;
        }
        .form-group label {
            display: block;
            margin-bottom: 0.5rem;
            font-weight: 500;
            color: var(--text);
        }
        .form-row {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 1rem;
        }
        .help-text {
            font-size: 0.85rem;
            color: var(--text-muted);
            margin-bottom: 0.5rem;
        }
        input[type="text"], input[type="password"], input[type="email"],
        input[type="number"], input[type="date"], textarea, select {
            width: 100%;
            padding: 0.75rem 1rem;
            background: rgba(15, 23, 42, 0.5);
            border: 1px solid var(--border);
            border-radius: var(--radius);
            color: var(--text);
            font-size: 1rem;
            font-family: inherit;
            transition: all 0.2s;
        }
        input:focus, textarea:focus, select:focus {
            outline: none;
            border-color: var(--primary);
            box-shadow: 0 0 0 3px rgba(99, 102, 241, 0.2);
        }
        textarea { resize: vertical; min-height: 100px; }
        .code-editor {
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.9rem;
        }
        input[type="color"] {
            width: 60px;
            height: 40px;
            padding: 0;
            border: none;
            cursor: pointer;
        }
        .color-grid {
            display: flex;
            gap: 1.5rem;
            flex-wrap: wrap;
        }
        .color-grid .form-group {
            text-align: center;
        }
        .checkbox-group {
            margin-top: 1rem;
        }
        .checkbox-label {
            display: flex;
            align-items: center;
            gap: 0.5rem;
            cursor: pointer;
        }
        .checkbox-label input[type="checkbox"] {
            width: 18px;
            height: 18px;
            cursor: pointer;
        }
        .form-actions {
            display: flex;
            gap: 1rem;
            justify-content: flex-end;
            margin-top: 2rem;
            padding-top: 1.5rem;
            border-top: 1px solid var(--border);
        }

        .field-row {
            display: grid;
            grid-template-columns: 1fr 1fr 120px 1fr 1fr auto auto;
            gap: 0.5rem;
            margin-bottom: 0.5rem;
            align-items: center;
        }
        .field-row input, .field-row select {
            padding: 0.5rem;
            font-size: 0.9rem;
        }

        .template-grid {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
            gap: 1.5rem;
        }
        .template-card {
            background: var(--bg-card);
            border: 1px solid var(--border);
            border-radius: var(--radius);
            padding: 1.5rem;
            transition: all 0.2s;
            text-decoration: none;
            color: var(--text);
            display: block;
        }
        .template-card:hover {
            border-color: var(--primary);
            transform: translateY(-2px);
            box-shadow: 0 10px 30px -10px rgba(99, 102, 241, 0.3);
            text-decoration: none;
        }
        .template-card h3 {
            margin-bottom: 0.5rem;
        }
        .template-card p {
            color: var(--text-muted);
            font-size: 0.9rem;
            margin-bottom: 1rem;
        }
        .template-category {
            display: inline-block;
            padding: 0.25rem 0.75rem;
            background: rgba(99, 102, 241, 0.2);
            border-radius: 9999px;
            font-size: 0.75rem;
            color: var(--primary);
        }

        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 1.5rem;
            margin-bottom: 2rem;
        }
        .stat-card {
            background: var(--bg-card);
            border: 1px solid var(--border);
            border-radius: var(--radius);
            padding: 1.5rem;
            display: flex;
            align-items: center;
            gap: 1rem;
        }
        .stat-icon {
            font-size: 2rem;
        }
        .stat-value {
            display: block;
            font-size: 2rem;
            font-weight: 700;
            font-family: 'Space Grotesk', sans-serif;
        }
        .stat-label {
            color: var(--text-muted);
            font-size: 0.9rem;
        }

        .quick-actions {
            margin-bottom: 2rem;
        }
        .quick-actions h2 {
            margin-bottom: 1rem;
            font-size: 1.25rem;
        }
        .action-buttons {
            display: flex;
            gap: 1rem;
            flex-wrap: wrap;
        }

        .recent-content h2 {
            margin-bottom: 1rem;
            font-size: 1.25rem;
        }
        .content-list {
            background: var(--bg-card);
            border: 1px solid var(--border);
            border-radius: var(--radius);
            overflow: hidden;
        }
        .content-item {
            display: grid;
            grid-template-columns: 1fr auto auto auto;
            gap: 1rem;
            padding: 1rem;
            border-bottom: 1px solid var(--border);
            color: var(--text);
            text-decoration: none;
            transition: background 0.2s;
            align-items: center;
        }
        .content-item:hover {
            background: var(--bg-hover);
            text-decoration: none;
        }
        .content-actions {
            display: flex;
            gap: 0.5rem;
        }
        .content-item:last-child {
            border-bottom: none;
        }
        .content-info {
            display: flex;
            flex-direction: column;
            gap: 0.25rem;
        }
        .content-title {
            font-weight: 500;
        }
        .content-slug {
            font-size: 0.75rem;
            color: var(--text-muted);
            font-family: 'JetBrains Mono', monospace;
        }
        .content-template {
            color: var(--text-muted);
            font-size: 0.9rem;
        }
        .content-status {
            padding: 0.25rem 0.75rem;
            border-radius: 9999px;
            font-size: 0.75rem;
        }
        .content-status.published {
            background: rgba(16, 185, 129, 0.2);
            color: var(--success);
        }
        .content-status.draft {
            background: rgba(148, 163, 184, 0.2);
            color: var(--text-muted);
        }

        .error-message {
            background: rgba(239, 68, 68, 0.1);
            border: 1px solid rgba(239, 68, 68, 0.3);
            color: #f87171;
            padding: 1rem;
            border-radius: var(--radius);
            margin-bottom: 1.5rem;
        }
        .success-message {
            background: rgba(16, 185, 129, 0.1);
            border: 1px solid rgba(16, 185, 129, 0.3);
            color: var(--success);
            padding: 1rem;
            border-radius: var(--radius);
            margin-bottom: 1.5rem;
        }

        .security-alert {
            background: linear-gradient(135deg, rgba(239, 68, 68, 0.15), rgba(251, 146, 60, 0.15));
            border: 2px solid #ef4444;
            border-radius: var(--radius);
            padding: 1.25rem;
            margin-bottom: 2rem;
            display: flex;
            align-items: center;
            gap: 1rem;
            animation: pulse-border 2s ease-in-out infinite;
        }
        @keyframes pulse-border {
            0%, 100% { border-color: #ef4444; }
            50% { border-color: #fb923c; }
        }
        .security-alert .alert-icon {
            font-size: 2rem;
        }
        .security-alert .alert-content {
            flex: 1;
            color: #fecaca;
        }
        .security-alert .alert-content strong {
            color: #fca5a5;
        }
        .security-alert .alert-content a {
            color: #fbbf24;
            font-weight: 600;
            text-decoration: underline;
        }
        .security-alert .alert-content a:hover {
            color: #fde047;
        }

        .password-requirements {
            list-style: none;
            padding: 0;
            color: var(--text-muted);
        }
        .password-requirements li {
            padding: 0.5rem 0;
            padding-left: 1.5rem;
            position: relative;
        }
        .password-requirements li::before {
            content: "•";
            position: absolute;
            left: 0;
            color: var(--primary);
        }

        @media (max-width: 768px) {
            .admin-layout {
                grid-template-columns: 1fr;
            }
            .sidebar {
                position: fixed;
                left: -100%;
                z-index: 100;
                transition: left 0.3s;
            }
            .sidebar.open {
                left: 0;
            }
            .field-row {
                grid-template-columns: 1fr 1fr;
            }
        }
    </style>
    <script>
    // Show info/alert modal (replacement for alert())
    function showAlert(message, title, callback) {
        var modal = document.getElementById('info-modal');
        var msgEl = document.getElementById('info-modal-message');
        var titleEl = document.getElementById('info-modal-title');
        var okBtn = document.getElementById('info-ok-btn');

        titleEl.textContent = title || 'Information';
        msgEl.innerHTML = message;
        modal.style.display = 'flex';

        function cleanup() {
            modal.style.display = 'none';
            okBtn.removeEventListener('click', onOk);
        }

        function onOk() {
            cleanup();
            if (callback) callback();
        }

        okBtn.addEventListener('click', onOk);
    }

    // Show confirm modal (replacement for confirm()) - returns a Promise
    function showConfirm(message, title) {
        return new Promise(function(resolve) {
            var modal = document.getElementById('confirm-modal');
            var msgEl = document.getElementById('confirm-modal-message');
            var titleEl = document.getElementById('confirm-modal-title');
            var okBtn = document.getElementById('confirm-ok-btn');
            var cancelBtn = document.getElementById('confirm-cancel-btn');

            titleEl.textContent = title || 'Confirm';
            msgEl.innerHTML = message;
            modal.style.display = 'flex';

            function cleanup() {
                modal.style.display = 'none';
                okBtn.removeEventListener('click', onOk);
                cancelBtn.removeEventListener('click', onCancel);
            }

            function onOk() {
                cleanup();
                resolve(true);
            }

            function onCancel() {
                cleanup();
                resolve(false);
            }

            okBtn.addEventListener('click', onOk);
            cancelBtn.addEventListener('click', onCancel);
        });
    }
    </script>
</head>
<body>
    <div class="admin-layout">
        <aside class="sidebar">
            <a href="/cm" class="sidebar-logo"><img src="/static/images/lightcms-logo.png" alt="LightCMS"></a>
            <nav>
                <div class="nav-section">
                    <div class="nav-section-title">Content</div>
                    <a href="/cm" class="nav-link">📊 Dashboard</a>
                    <a href="/cm/content" class="nav-link">📄 Content</a>
                    <a href="/cm/templates" class="nav-link">📋 Templates</a>
                    <a href="/cm/collections" class="nav-link">📁 Collections</a>
                    <a href="/cm/folders" class="nav-link">🗂️ Folders</a>
                </div>
                <div class="nav-section">
                    <div class="nav-section-title">Media</div>
                    <a href="/cm/assets" class="nav-link">🖼️ Asset Library</a>
                </div>
                <div class="nav-section">
                    <div class="nav-section-title">Settings</div>
                    <a href="/cm/theme" class="nav-link">🎨 Theme</a>
                    <a href="/cm/redirects" class="nav-link">↪️ Redirects</a>
                    <a href="/cm/config" class="nav-link">⚙️ Configuration</a>
                    <a href="/cm/security" class="nav-link">🔒 Security</a>
                </div>
                <div class="nav-section">
                    <div class="nav-section-title">Tools</div>
                    <a href="/cm/tools/broken-links" class="nav-link">🔗 Broken Link Finder</a>
                </div>
                <div class="nav-section">
                    <div class="nav-section-title">Inbox</div>
                    <a href="/cm/messages" class="nav-link">📬 Messages{{if .UnreadMessageCount}} <span class="nav-badge">{{.UnreadMessageCount}}</span>{{end}}</a>
                </div>
                <div class="nav-section">
                    <a href="/" target="_blank" class="nav-link">🌐 View Site</a>
                    <form method="POST" action="/cm/logout" style="margin: 0;">
                        {{.CSRFField}}
                        <button type="submit" class="nav-link logout-btn">🚪 Logout</button>
                    </form>
                </div>
            </nav>
        </aside>
        <main class="main-content">`

const adminLayoutEnd = `
        </main>
    </div>

    <!-- Delete confirmation modal -->
    <div id="delete-modal" style="display: none; position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0, 0, 0, 0.7); z-index: 10000; align-items: center; justify-content: center;">
        <div style="background: #1e293b; border-radius: var(--radius); max-width: 450px; width: 90%; box-shadow: 0 20px 60px rgba(0, 0, 0, 0.8); border: 1px solid rgba(239, 68, 68, 0.3);">
            <div style="padding: 1.5rem; border-bottom: 1px solid rgba(239, 68, 68, 0.2); background: #1a2332;">
                <h3 style="margin: 0; color: var(--danger);">Confirm Delete</h3>
            </div>
            <div style="padding: 1.5rem; background: #1e293b;">
                <p id="delete-modal-message" style="margin: 0;">Are you sure you want to delete this item?</p>
            </div>
            <div style="padding: 1rem 1.5rem; border-top: 1px solid rgba(239, 68, 68, 0.2); display: flex; gap: 0.75rem; justify-content: flex-end; background: #1a2332;">
                <button type="button" class="btn btn-outline" id="delete-cancel-btn">Cancel</button>
                <button type="button" class="btn" id="delete-confirm-btn" style="background: var(--danger); color: white;">Delete</button>
            </div>
        </div>
    </div>

    <!-- Info/Alert modal -->
    <div id="info-modal" style="display: none; position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0, 0, 0, 0.7); z-index: 10000; align-items: center; justify-content: center;">
        <div style="background: #1e293b; border-radius: var(--radius); max-width: 450px; width: 90%; box-shadow: 0 20px 60px rgba(0, 0, 0, 0.8); border: 1px solid rgba(99, 102, 241, 0.3);">
            <div style="padding: 1.5rem; border-bottom: 1px solid rgba(99, 102, 241, 0.2); background: #1a2332;">
                <h3 id="info-modal-title" style="margin: 0; color: var(--primary);">Information</h3>
            </div>
            <div style="padding: 1.5rem; background: #1e293b;">
                <p id="info-modal-message" style="margin: 0;"></p>
            </div>
            <div style="padding: 1rem 1.5rem; border-top: 1px solid rgba(99, 102, 241, 0.2); display: flex; gap: 0.75rem; justify-content: flex-end; background: #1a2332;">
                <button type="button" class="btn" id="info-ok-btn" style="background: var(--primary); color: white;">OK</button>
            </div>
        </div>
    </div>

    <!-- Confirm modal -->
    <div id="confirm-modal" style="display: none; position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0, 0, 0, 0.7); z-index: 10000; align-items: center; justify-content: center;">
        <div style="background: #1e293b; border-radius: var(--radius); max-width: 500px; width: 90%; box-shadow: 0 20px 60px rgba(0, 0, 0, 0.8); border: 1px solid rgba(99, 102, 241, 0.3);">
            <div style="padding: 1.5rem; border-bottom: 1px solid rgba(99, 102, 241, 0.2); background: #1a2332;">
                <h3 id="confirm-modal-title" style="margin: 0; color: var(--primary);">Confirm</h3>
            </div>
            <div style="padding: 1.5rem; background: #1e293b;">
                <p id="confirm-modal-message" style="margin: 0;"></p>
            </div>
            <div style="padding: 1rem 1.5rem; border-top: 1px solid rgba(99, 102, 241, 0.2); display: flex; gap: 0.75rem; justify-content: flex-end; background: #1a2332;">
                <button type="button" class="btn btn-outline" id="confirm-cancel-btn">Cancel</button>
                <button type="button" class="btn" id="confirm-ok-btn" style="background: var(--primary); color: white;">Confirm</button>
            </div>
        </div>
    </div>

    <!-- Revert confirmation modal -->
    <div id="revert-modal" style="display: none; position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0, 0, 0, 0.7); z-index: 10000; align-items: center; justify-content: center;">
        <div style="background: #1e293b; border-radius: var(--radius); max-width: 450px; width: 90%; box-shadow: 0 20px 60px rgba(0, 0, 0, 0.8); border: 1px solid rgba(245, 158, 11, 0.3);">
            <div style="padding: 1.5rem; border-bottom: 1px solid rgba(245, 158, 11, 0.2); background: #1a2332;">
                <h3 style="margin: 0; color: var(--warning);">Confirm Revert</h3>
            </div>
            <div style="padding: 1.5rem; background: #1e293b;">
                <p id="revert-modal-message" style="margin: 0;"></p>
            </div>
            <div style="padding: 1rem 1.5rem; border-top: 1px solid rgba(245, 158, 11, 0.2); display: flex; gap: 0.75rem; justify-content: flex-end; background: #1a2332;">
                <button type="button" class="btn btn-outline" id="revert-cancel-btn">Cancel</button>
                <button type="button" class="btn" id="revert-confirm-btn" style="background: var(--warning); color: white;">Revert</button>
            </div>
        </div>
    </div>

    <script>
    // Revert confirmation modal
    var revertModalForm = null;
    function confirmRevert(form, version) {
        revertModalForm = form;
        var modal = document.getElementById('revert-modal');
        var msgEl = document.getElementById('revert-modal-message');
        var confirmBtn = document.getElementById('revert-confirm-btn');
        var cancelBtn = document.getElementById('revert-cancel-btn');

        msgEl.innerHTML = 'Revert to version ' + version + '?<br><br>A new version will be saved with the reverted content.';
        modal.style.display = 'flex';

        function cleanup() {
            modal.style.display = 'none';
            confirmBtn.removeEventListener('click', onConfirm);
            cancelBtn.removeEventListener('click', onCancel);
        }

        function onConfirm() {
            cleanup();
            if (revertModalForm) {
                revertModalForm.submit();
            }
        }

        function onCancel() {
            cleanup();
            revertModalForm = null;
        }

        confirmBtn.addEventListener('click', onConfirm);
        cancelBtn.addEventListener('click', onCancel);

        return false; // Prevent form submission
    }

    // Delete confirmation modal
    var deleteModalForm = null;
    function confirmDelete(form, message) {
        deleteModalForm = form;
        var modal = document.getElementById('delete-modal');
        var msgEl = document.getElementById('delete-modal-message');
        var confirmBtn = document.getElementById('delete-confirm-btn');
        var cancelBtn = document.getElementById('delete-cancel-btn');

        msgEl.innerHTML = message || 'Are you sure you want to delete this item?';
        modal.style.display = 'flex';

        function cleanup() {
            modal.style.display = 'none';
            confirmBtn.removeEventListener('click', onConfirm);
            cancelBtn.removeEventListener('click', onCancel);
        }

        function onConfirm() {
            cleanup();
            if (deleteModalForm) {
                deleteModalForm.submit();
            }
        }

        function onCancel() {
            cleanup();
            deleteModalForm = null;
        }

        confirmBtn.addEventListener('click', onConfirm);
        cancelBtn.addEventListener('click', onCancel);

        return false; // Prevent form submission
    }
    </script>
</body>
</html>`
