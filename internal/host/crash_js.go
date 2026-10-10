//go:build js

package host

import "syscall/js"

// crashKey is where the browser keeps the last crash report, to read after a reload.
const crashKey = "uw1go-crash"

// ReportCrash shows the report in a panel over the game, with a button that copies it, and
// keeps it in the browser's storage under crashKey.
func ReportCrash(report string) {
	g := js.Global()
	g.Get("console").Call("error", report)
	if ls := g.Get("localStorage"); !ls.IsUndefined() && !ls.IsNull() {
		ls.Call("setItem", crashKey, report)
	}
	doc := g.Get("document")
	if doc.IsUndefined() || doc.Get("body").IsNull() {
		return
	}
	el := func(tag, style, text string) js.Value {
		e := doc.Call("createElement", tag)
		e.Set("style", style)
		e.Set("textContent", text)
		return e
	}
	box := el("div", "position:fixed;inset:16px;z-index:99999;display:flex;flex-direction:column;gap:8px;"+
		"padding:12px;background:#111;color:#eee;font:13px/1.4 sans-serif;border:1px solid #555", "")
	box.Call("appendChild", el("div", "font-weight:bold",
		"The game crashed. Copy this report and send it along (it is also kept in this browser as localStorage \""+crashKey+"\")."))
	pre := el("pre", "flex:1;overflow:auto;margin:0;padding:8px;background:#000;font:12px/1.4 monospace;"+
		"white-space:pre-wrap;user-select:text", report)
	btn := el("button", "align-self:flex-start;padding:6px 14px;font:inherit;cursor:pointer", "Copy report")
	btn.Call("addEventListener", "click", js.FuncOf(func(js.Value, []js.Value) any {
		if c := g.Get("navigator").Get("clipboard"); !c.IsUndefined() {
			c.Call("writeText", report)
			btn.Set("textContent", "Copied")
		} else {
			sel := g.Call("getSelection")
			sel.Call("selectAllChildren", pre)
		}
		return nil
	}))
	box.Call("appendChild", btn)
	box.Call("appendChild", pre)
	doc.Get("body").Call("appendChild", box)
	if g.Get("document").Get("pointerLockElement").Truthy() {
		doc.Call("exitPointerLock")
	}
}
