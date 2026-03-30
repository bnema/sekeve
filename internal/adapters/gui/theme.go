// internal/adapters/gui/theme.go
//go:build linux && !nogtk

package gui

const emeraldCSS = `
window {
    background-color: transparent;
}

* {
    outline-style: none;
    outline-width: 0;
    outline-color: transparent;
}

*:focus, *:focus-visible, *:focus-within {
    outline-style: none;
    outline-width: 0;
    outline-color: transparent;
}

.sekeve-overlay {
    background-color: #0f1a16;
    border: 1px solid #1a3028;
    border-radius: 10px;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.5), 0 0 16px rgba(52, 211, 153, 0.08);
    font-size: 14px;
}

.sekeve-header {
    background-color: #0d1814;
    border-bottom: 1px solid #1a3028;
    padding: 6px 10px;
}

.sekeve-tab {
    padding: 4px 14px;
    border-radius: 6px;
    font-size: 13px;
    font-weight: 500;
    color: #4a7a66;
    background: transparent;
    border: none;
}

.sekeve-tab-active {
    background-color: #34d399;
    color: #0f1a16;
}

.sekeve-category-bar {
    border-bottom: 1px solid #1a3028;
    padding: 6px 10px;
}

.sekeve-category {
    padding: 3px 10px;
    border-radius: 4px;
    font-size: 12px;
    font-weight: 500;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: #3a6a56;
    background: transparent;
    border: none;
}

.sekeve-category-active {
    background-color: rgba(52, 211, 153, 0.12);
    color: #34d399;
}

.sekeve-search-row {
    border-bottom: 1px solid #1a3028;
    padding: 0 12px;
}

.sekeve-search-row entry, .sekeve-search-row searchentry {
    background: transparent;
    border: none;
    box-shadow: none;
    padding: 10px 0;
    font-size: 14px;
    color: #c8e8d8;
}

.sekeve-search-row entry:focus, .sekeve-search-row searchentry:focus {
    border: none;
    box-shadow: none;
}

entry, passwordentry {
    background-color: #0a140f;
    color: #c8e8d8;
    border: 1px solid #1a3028;
    border-radius: 6px;
    padding: 8px 10px;
    font-size: 14px;
}

entry:focus, passwordentry:focus {
    border-color: #34d399;
    box-shadow: 0 0 0 1px rgba(52, 211, 153, 0.2);
}

textview {
    background-color: #0a140f;
    color: #c8e8d8;
    font-size: 14px;
}

textview border {
    border: 1px solid #1a3028;
    border-radius: 6px;
}

textview:focus border {
    border-color: #34d399;
}

.sekeve-label {
    font-size: 12px;
    font-weight: 500;
    color: #4a7a66;
    text-transform: uppercase;
    letter-spacing: 0.04em;
}

list, listview, listbox {
    background-color: transparent;
}

row, list row, listbox row {
    background-color: transparent;
}

.sekeve-row {
    padding: 7px 12px;
    background-color: transparent;
}

.sekeve-row:selected, .sekeve-row-selected {
    background-color: rgba(52, 211, 153, 0.06);
}

.sekeve-row-name {
    font-weight: 500;
    font-size: 14px;
    color: #b0d8c8;
}

.sekeve-row:selected .sekeve-row-name {
    color: #34d399;
}

.sekeve-row-meta {
    font-size: 12px;
    color: #3a6a56;
}

.sekeve-footer {
    border-top: 1px solid #1a3028;
    font-size: 11px;
    color: #2a5040;
    padding: 6px 12px;
}

.sekeve-kbd {
    background-color: rgba(52, 211, 153, 0.08);
    color: #4a7a66;
    padding: 1px 5px;
    border-radius: 3px;
    font-size: 11px;
}

.sekeve-btn-cancel {
    background-color: #1a3028;
    color: #4a7a66;
    border: none;
    border-radius: 6px;
    padding: 6px 18px;
    font-size: 13px;
}

.sekeve-btn-save {
    background-color: #34d399;
    color: #0f1a16;
    border: none;
    border-radius: 6px;
    padding: 6px 18px;
    font-size: 13px;
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
.sekeve-pin {
    background-color: #0f1a16;
    border: 1px solid #1a3028;
    border-radius: 10px;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.5), 0 0 16px rgba(52, 211, 153, 0.08);
}

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
