// internal/adapters/gui/theme.go
//go:build linux

package gui

const emeraldCSS = `
window {
    background-color: transparent;
}

.sekeve-overlay {
    background-color: #0f1a16;
    border: 1px solid #1a3028;
    border-radius: 10px;
}

.sekeve-header {
    background-color: #0d1814;
    border-bottom: 1px solid #1a3028;
}

.sekeve-tab {
    padding: 4px 14px;
    border-radius: 6px;
    font-size: 12px;
    font-weight: 500;
    color: #4a7a66;
    background: transparent;
    border: none;
}

.sekeve-tab-active {
    background-color: #34d399;
    color: #0f1a16;
}

.sekeve-category {
    padding: 3px 10px;
    border-radius: 4px;
    font-size: 11px;
    font-weight: 500;
    color: #3a6a56;
    background: transparent;
    border: none;
}

.sekeve-category-active {
    background-color: rgba(52, 211, 153, 0.12);
    color: #34d399;
}

entry, passwordentry {
    background-color: #0a140f;
    color: #c8e8d8;
    border: 1px solid #1a3028;
    border-radius: 6px;
    padding: 8px 10px;
    font-size: 13px;
}

entry:focus, passwordentry:focus {
    border-color: #34d399;
    box-shadow: 0 0 0 1px rgba(52, 211, 153, 0.2);
}

textview {
    background-color: #0a140f;
    color: #c8e8d8;
    font-size: 13px;
}

textview border {
    border: 1px solid #1a3028;
    border-radius: 6px;
}

textview:focus border {
    border-color: #34d399;
}

.sekeve-label {
    font-size: 11px;
    font-weight: 500;
    color: #4a7a66;
}

.sekeve-row {
    padding: 7px 12px;
}

.sekeve-row:selected, .sekeve-row-selected {
    background-color: rgba(52, 211, 153, 0.06);
}

.sekeve-row-name {
    font-weight: 500;
    color: #b0d8c8;
}

.sekeve-row:selected .sekeve-row-name {
    color: #34d399;
}

.sekeve-row-meta {
    font-size: 11px;
    color: #3a6a56;
}

.sekeve-footer {
    border-top: 1px solid #1a3028;
    font-size: 10px;
    color: #2a5040;
    padding: 6px 12px;
}

.sekeve-btn-cancel {
    background-color: #1a3028;
    color: #4a7a66;
    border: none;
    border-radius: 6px;
    padding: 6px 18px;
    font-size: 12px;
}

.sekeve-btn-save {
    background-color: #34d399;
    color: #0f1a16;
    border: none;
    border-radius: 6px;
    padding: 6px 18px;
    font-size: 12px;
}

.sekeve-copy-btn {
    background-color: #1a3028;
    border: 1px solid #1a3028;
    border-radius: 6px;
    color: #4a7a66;
    padding: 7px 8px;
}

.sekeve-copy-btn:hover {
    color: #34d399;
    border-color: #34d399;
}

.sekeve-icon-login { color: #34d399; }
.sekeve-icon-note { color: #6ee7b7; }
.sekeve-icon-secret { color: #a7f3d0; }
.sekeve-icon-search { color: #3a6a56; }

/* PIN prompt overrides */
.sekeve-pin entry, .sekeve-pin passwordentry {
    font-size: 20px;
    min-width: 320px;
    padding: 8px 12px;
}

.sekeve-pin-error entry, .sekeve-pin-error passwordentry {
    border-color: #E5484D;
    background-color: #2A2020;
}

.sekeve-pin-error .sekeve-label {
    color: #E5484D;
}
`
