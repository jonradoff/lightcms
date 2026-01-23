package handlers

var adminTemplates = map[string]string{
	"login": `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Login - LightCMS</title>
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
            font-family: 'Space Grotesk', sans-serif;
            font-size: 2rem;
            font-weight: 700;
            background: linear-gradient(135deg, #6366f1, #8b5cf6, #06b6d4);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            text-align: center;
            margin-bottom: 0.5rem;
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
        <h1 class="logo">LightCMS</h1>
        <p class="subtitle">Content Management System</p>
        {{if .Error}}<div class="error">{{.Error}}</div>{{end}}
        <form method="POST" action="/cm/login" autocomplete="off">
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
                            <form method="POST" action="/cm/templates/{{.ID.Hex}}/delete" onsubmit="return confirm('Delete this template?')">
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
            <h1>Content{{if .ShowDeleted}} <span style="color: var(--danger);">(Deleted)</span>{{end}}</h1>
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
                {{if .ShowDeleted}}
                <a href="/cm/content" class="btn btn-sm btn-outline">← Show Active</a>
                {{else}}
                <a href="/cm/content?deleted=true" class="btn btn-sm btn-outline" style="border-color: var(--danger); color: var(--danger);">View Deleted</a>
                {{end}}
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
                                <button type="submit" class="btn btn-sm btn-primary">Restore</button>
                            </form>
                            {{else}}
                            <a href="{{if .FullPath}}{{.FullPath}}{{else}}/{{.Slug}}{{end}}" target="_blank" class="btn btn-sm btn-outline">View</a>
                            <a href="/cm/content/{{.ID.Hex}}" class="btn btn-sm">Edit</a>
                            <form method="POST" action="/cm/content/{{.ID.Hex}}/regenerate" style="display:inline">
                                <button type="submit" class="btn btn-sm btn-secondary" title="Regenerate static file">↻</button>
                            </form>
                            <form method="POST" action="/cm/content/{{.ID.Hex}}/delete" onsubmit="return confirm('Delete this content?')">
                                <button type="submit" class="btn btn-sm btn-danger">Delete</button>
                            </form>
                            {{end}}
                        </td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>
        <style>
            code {
                font-family: 'JetBrains Mono', monospace;
                background: rgba(99, 102, 241, 0.1);
                padding: 0.25rem 0.5rem;
                border-radius: 4px;
                font-size: 0.85rem;
            }
        </style>
        <script>
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
            <div class="diff-section">
                <h3>Title</h3>
                <div class="diff-row">
                    <div class="diff-col diff-old">
                        <div class="diff-label">Version {{.Version.Version}}</div>
                        <div class="diff-content">{{.Version.Title}}</div>
                    </div>
                    <div class="diff-col diff-new">
                        <div class="diff-label">Current</div>
                        <div class="diff-content">{{.Current.Title}}</div>
                    </div>
                </div>
            </div>

            <!-- Slug -->
            <div class="diff-section">
                <h3>Slug</h3>
                <div class="diff-row">
                    <div class="diff-col diff-old">
                        <div class="diff-label">Version {{.Version.Version}}</div>
                        <div class="diff-content"><code>{{.Version.Slug}}</code></div>
                    </div>
                    <div class="diff-col diff-new">
                        <div class="diff-label">Current</div>
                        <div class="diff-content"><code>{{.Current.Slug}}</code></div>
                    </div>
                </div>
            </div>

            <!-- Full Path -->
            <div class="diff-section">
                <h3>Full Path</h3>
                <div class="diff-row">
                    <div class="diff-col diff-old">
                        <div class="diff-label">Version {{.Version.Version}}</div>
                        <div class="diff-content"><code>{{.Version.FullPath}}</code></div>
                    </div>
                    <div class="diff-col diff-new">
                        <div class="diff-label">Current</div>
                        <div class="diff-content"><code>{{.Current.FullPath}}</code></div>
                    </div>
                </div>
            </div>

            <!-- Content Data Fields -->
            {{range $key, $value := .Version.Data}}
            <div class="diff-section">
                <h3>{{$key}}</h3>
                <div class="diff-row">
                    <div class="diff-col diff-old">
                        <div class="diff-label">Version {{$.Version.Version}}</div>
                        <div class="diff-content diff-html">{{$value}}</div>
                    </div>
                    <div class="diff-col diff-new">
                        <div class="diff-label">Current</div>
                        <div class="diff-content diff-html">{{index $.Current.Data $key}}</div>
                    </div>
                </div>
            </div>
            {{end}}

            <!-- Settings -->
            <div class="diff-section">
                <h3>Settings</h3>
                <div class="diff-row">
                    <div class="diff-col diff-old">
                        <div class="diff-label">Version {{.Version.Version}}</div>
                        <div class="diff-content">
                            <p>Published: {{if .Version.Published}}Yes{{else}}No{{end}}</p>
                            <p>Use Header: {{if .Version.UseHeader}}Yes{{else}}No{{end}}</p>
                            <p>Use Footer: {{if .Version.UseFooter}}Yes{{else}}No{{end}}</p>
                            <p>Use Theme: {{if .Version.UseTheme}}Yes{{else}}No{{end}}</p>
                        </div>
                    </div>
                    <div class="diff-col diff-new">
                        <div class="diff-label">Current</div>
                        <div class="diff-content">
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
            <form method="POST" action="/cm/content/{{.Current.ID.Hex}}/versions/{{.Version.Version}}/revert" style="display:inline" onsubmit="return confirm('Revert to version {{.Version.Version}}? A new version will be saved with the reverted content.')">
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
            .diff-section h3 {
                margin: 0 0 1rem 0;
                font-size: 1rem;
                color: var(--accent);
                border-bottom: 1px solid var(--border);
                padding-bottom: 0.5rem;
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
            @media (max-width: 768px) {
                .diff-row {
                    grid-template-columns: 1fr;
                }
            }
        </style>
    ` + adminLayoutEnd,

	"content_form": adminLayoutStart + `
        <div class="page-header">
            <h1>{{if .IsNew}}New {{.Template.Name}}{{else}}Edit Content{{end}}</h1>
        </div>
        {{if .Error}}<div class="error-message">{{.Error}}</div>{{end}}
        <form method="POST" action="{{if .IsNew}}/cm/content/create{{else}}/cm/content/{{.Content.ID.Hex}}{{end}}" enctype="multipart/form-data" class="form-card">
            <input type="hidden" name="template_id" value="{{.Template.ID.Hex}}">

            <div class="form-group">
                <label for="title">Title</label>
                <input type="text" id="title" name="title" value="{{if .Content}}{{.Content.Title}}{{end}}" required>
            </div>
            <div class="form-group">
                <label for="slug">Slug (URL path)</label>
                <input type="text" id="slug" name="slug" value="{{if .Content}}{{.Content.Slug}}{{end}}" placeholder="auto-generated from title">
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

        {{if not .IsNew}}
        {{if .Content.Deleted}}
        <div class="form-section" style="margin-top: 2rem; background: rgba(239, 68, 68, 0.1); border: 1px solid rgba(239, 68, 68, 0.3); border-radius: var(--radius); padding: 1.5rem;">
            <h3 style="color: var(--danger);">⚠️ This content is deleted</h3>
            <p style="margin-bottom: 1rem;">This page was deleted on {{if .Content.DeletedAt}}{{.Content.DeletedAt.Format "Jan 2, 2006 3:04 PM"}}{{else}}unknown date{{end}}.</p>
            <form method="POST" action="/cm/content/{{.Content.ID.Hex}}/undelete" style="display: inline;">
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
                            <th>Saved</th>
                            <th>Actions</th>
                        </tr>
                    </thead>
                    <tbody>
                        {{range .Versions}}
                        <tr>
                            <td>v{{.Version}}</td>
                            <td>{{.Title}}</td>
                            <td>{{.CreatedAt.Format "Jan 2, 2006 3:04 PM"}}</td>
                            <td class="actions">
                                <a href="/cm/content/{{.ContentID.Hex}}/versions/{{.Version}}/diff" class="btn btn-sm btn-outline">Diff</a>
                                <a href="/cm/content/{{.ContentID.Hex}}/versions/{{.Version}}/view" target="_blank" class="btn btn-sm btn-outline">Preview</a>
                                <form method="POST" action="/cm/content/{{.ContentID.Hex}}/versions/{{.Version}}/revert" style="display:inline" onsubmit="return confirm('Revert to version {{.Version}}? A new version will be saved with the reverted content.')">
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

            // If no slug, generate from title
            if (!slug && titleInput && titleInput.value) {
                slug = titleInput.value.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');
            }

            var fullPath = folderPath ? folderPath + '/' + slug : '/' + slug;
            if (!slug) fullPath = folderPath || '/';

            preview.textContent = fullPath;
        }

        // Set up event listeners for path preview
        document.addEventListener('DOMContentLoaded', function() {
            var folderSelect = document.getElementById('folder_id');
            var slugInput = document.getElementById('slug');
            var titleInput = document.getElementById('title');

            if (folderSelect) folderSelect.addEventListener('change', updatePathPreview);
            if (slugInput) slugInput.addEventListener('input', updatePathPreview);
            if (titleInput) titleInput.addEventListener('input', updatePathPreview);

            // Initial preview
            updatePathPreview();
        });
        </script>
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
                            <form method="POST" action="/cm/collections/{{.ID.Hex}}/delete" onsubmit="return confirm('Delete this collection?')">
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
        </div>
        {{if .Error}}<div class="error-message">{{.Error}}</div>{{end}}
        {{if .Success}}<div class="success-message">{{.Success}}</div>{{end}}
        <form method="POST" class="form-card">
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

            <div class="form-group">
                <label for="custom_css">Custom CSS</label>
                <textarea id="custom_css" name="custom_css" rows="10" class="code-editor">{{.Settings.CustomCSS}}</textarea>
            </div>

            <div class="form-section">
                <h3>Site Header</h3>
                <p class="help-text">HTML content displayed at the top of every page (that has header enabled). Use navigation links, logo, etc. Internal links will be automatically updated if page slugs change.</p>
                <div class="form-group">
                    <textarea id="header_html" name="header_html" rows="12" class="code-editor">{{.Settings.HeaderHTML}}</textarea>
                </div>
            </div>

            <div class="form-section">
                <h3>Site Footer</h3>
                <p class="help-text">HTML content displayed at the bottom of every page (that has footer enabled). Internal links will be automatically updated if page slugs change.</p>
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
                            <form method="POST" action="/cm/folders/{{.Folder.ID.Hex}}/delete" onsubmit="return confirm('Delete this folder? Make sure it has no content or subfolders.')">
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
                        <span class="version-value">{{.SoftwareVersion}}</span>
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
        <div class="table-card">
            <table class="data-table">
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
                            <form method="POST" action="/cm/redirects/{{.ID.Hex}}/delete" style="display:inline" onsubmit="return confirm('Delete this redirect?\n\nNote: Browsers cache 301 redirects. After deletion, users may need to clear their browser cache or use incognito mode to see the change.')">
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
                            <form method="POST" action="/cm/messages/{{.ID.Hex}}/delete" style="display:inline" onsubmit="return confirm('Delete this message?')">
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
            <form method="POST" action="/cm/messages/{{.Message.ID.Hex}}/delete" style="display:inline" onsubmit="return confirm('Delete this message?')">
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
            <table class="data-table">
                <thead>
                    <tr>
                        <th>Preview</th>
                        <th>Filename</th>
                        <th>Folder</th>
                        <th>URL Path</th>
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
                            <img src="/assets{{.FullPath}}" alt="{{.Filename}}" style="max-width: 50px; max-height: 50px; border-radius: 4px;">
                            {{else}}
                            <span style="font-size: 1.5rem;">📄</span>
                            {{end}}
                        </td>
                        <td>{{.Filename}}</td>
                        <td><code>{{.Folder}}</code></td>
                        <td><code>/assets{{.FullPath}}</code></td>
                        <td>{{.MimeType}}</td>
                        <td>{{if lt .Size 1024}}{{.Size}} B{{else if lt .Size 1048576}}{{divide .Size 1024}} KB{{else}}{{divide .Size 1048576}} MB{{end}}</td>
                        <td>
                            <a href="/assets{{.FullPath}}" target="_blank" class="btn btn-small">View</a>
                            <button onclick="copyToClipboard('/assets{{.FullPath}}')" class="btn btn-small btn-secondary">Copy URL</button>
                            <form method="POST" action="/cm/assets/{{.ID.Hex}}/delete" style="display:inline" onsubmit="return confirm('Delete this asset?')">
                                <button type="submit" class="btn btn-small btn-danger">Delete</button>
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

	"asset_upload": adminLayoutStart + `
        <div class="page-header">
            <h1>Upload Asset</h1>
            <a href="/cm/assets" class="btn btn-secondary">Back to Library</a>
        </div>

        <form method="POST" action="/cm/assets/upload" enctype="multipart/form-data" class="form-card">
            <div class="form-group">
                <label for="file">File</label>
                <input type="file" id="file" name="file" required style="padding: 0.75rem; background: var(--bg-dark); border: 1px solid rgba(255,255,255,0.1); border-radius: 8px; color: var(--text);">
            </div>

            <div class="form-group">
                <label for="filename">Filename (optional, defaults to uploaded filename)</label>
                <input type="text" id="filename" name="filename" placeholder="e.g., logo.png">
            </div>

            <div class="form-group">
                <label for="folder">Folder</label>
                <input type="text" id="folder" name="folder" value="/" placeholder="e.g., /images or /documents" list="folder-list">
                <datalist id="folder-list">
                    {{range .Folders}}
                    <option value="{{.}}">
                    {{end}}
                </datalist>
                <small style="color: var(--text-muted);">Use / for root, or create folders like /images, /documents</small>
            </div>

            <div class="form-group">
                <label for="description">Description (optional)</label>
                <input type="text" id="description" name="description" placeholder="Brief description of the asset">
            </div>

            <div class="form-actions">
                <button type="submit" class="btn btn-primary">Upload Asset</button>
            </div>
        </form>
    ` + adminLayoutEnd,
}

const adminLayoutStart = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>LightCMS Admin</title>
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
            font-family: 'Space Grotesk', sans-serif;
            font-size: 1.5rem;
            font-weight: 700;
            background: linear-gradient(135deg, var(--primary), var(--secondary), var(--accent));
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            margin-bottom: 2rem;
            display: block;
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
</head>
<body>
    <div class="admin-layout">
        <aside class="sidebar">
            <a href="/cm" class="sidebar-logo">LightCMS</a>
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
                    <div class="nav-section-title">Inbox</div>
                    <a href="/cm/messages" class="nav-link">📬 Messages{{if .UnreadMessageCount}} <span class="nav-badge">{{.UnreadMessageCount}}</span>{{end}}</a>
                </div>
                <div class="nav-section">
                    <a href="/" target="_blank" class="nav-link">🔗 View Site</a>
                    <a href="/cm/logout" class="nav-link">🚪 Logout</a>
                </div>
            </nav>
        </aside>
        <main class="main-content">`

const adminLayoutEnd = `
        </main>
    </div>
</body>
</html>`
