(function () {
  "use strict";

  var API = "/api/v1";
  var store = window.sessionStorage;
  var $ = function (sel) { return document.querySelector(sel); };

  // ---------- helpers ----------
  function el(tag, props, children) {
    var n = document.createElement(tag);
    Object.keys(props || {}).forEach(function (k) {
      if (k === "text") n.textContent = props[k];
      else if (k === "class") n.className = props[k];
      else n.setAttribute(k, props[k]);
    });
    (children || []).forEach(function (c) { n.appendChild(c); });
    return n;
  }
  function msg(text, ok) {
    var m = $("#msg");
    if (!m) return;
    m.textContent = text || "";
    m.className = "msg" + (ok ? " ok" : "");
  }
  function saveTokens(t) {
    store.setItem("access_token", t.access_token);
    store.setItem("refresh_token", t.refresh_token);
  }
  function clearSession() {
    store.removeItem("access_token");
    store.removeItem("refresh_token");
    store.removeItem("me");
  }
  function goLogin() { window.location.href = "/admin-ui"; }

  function rawFetch(method, path, body, token) {
    var headers = {};
    if (body !== undefined) headers["Content-Type"] = "application/json";
    if (token) headers["Authorization"] = "Bearer " + token;
    return fetch(API + path, {
      method: method, headers: headers, body: body === undefined ? undefined : JSON.stringify(body),
    }).then(function (r) {
      if (r.status === 204) return { ok: true, status: 204, data: null };
      return r.json().catch(function () { return {}; }).then(function (d) { return { ok: r.ok, status: r.status, data: d }; });
    });
  }

  var refreshing = null;
  function refresh() {
    if (!refreshing) {
      var rt = store.getItem("refresh_token");
      refreshing = (rt ? rawFetch("POST", "/auth/refresh", { refresh_token: rt }) : Promise.resolve({ ok: false }))
        .then(function (res) {
          if (res.ok) { saveTokens(res.data); return true; }
          return false;
        })
        .then(function (ok) { refreshing = null; return ok; });
    }
    return refreshing;
  }

  // api() resolves with data or rejects with an Error whose message is human readable.
  function api(method, path, body) {
    function attempt(retry) {
      return rawFetch(method, path, body, store.getItem("access_token")).then(function (res) {
        if (res.status === 401 && retry) {
          return refresh().then(function (ok) {
            if (ok) return attempt(false);
            clearSession(); goLogin();
            throw new Error("Session expired");
          });
        }
        if (!res.ok) {
          var e = res.data && res.data.error;
          throw new Error(e ? e.message : "Request failed (" + res.status + ")");
        }
        return res.data;
      });
    }
    return attempt(true);
  }
  function fail(err) { msg(err.message); }

  // ---------- shared chrome ----------
  function initChrome() {
    var page = document.body.getAttribute("data-page");
    document.querySelectorAll("[data-nav]").forEach(function (a) {
      if (a.getAttribute("data-nav") === page) a.classList.add("active");
    });
    var out = $("#logout");
    if (out) out.addEventListener("click", function () {
      var rt = store.getItem("refresh_token");
      (rt ? rawFetch("POST", "/auth/logout", { refresh_token: rt }) : Promise.resolve()).then(function () {
        clearSession(); goLogin();
      });
    });
    var me = null;
    try { me = JSON.parse(store.getItem("me") || "null"); } catch (e) { me = null; }
    if (!store.getItem("access_token") || !me || me.role !== "admin") { clearSession(); goLogin(); return false; }
    var who = $("#whoami");
    if (who) who.textContent = me.email;
    return true;
  }

  // ---------- login ----------
  function initLogin() {
    var loginForm = $("#login-form"), mfaForm = $("#mfa-form"), mfaToken = null;

    function finish(auth) {
      if (auth.user.role !== "admin") { msg("This account is not an administrator."); return; }
      saveTokens(auth.tokens);
      store.setItem("me", JSON.stringify(auth.user));
      window.location.href = "/admin-ui/dashboard";
    }
    function handle(res) {
      if (!res.ok) { msg(res.data && res.data.error ? res.data.error.message : "Sign in failed"); return; }
      if (res.data.mfa_required) {
        mfaToken = res.data.mfa_token;
        loginForm.classList.add("hidden");
        mfaForm.classList.remove("hidden");
        msg("");
        return;
      }
      finish(res.data);
    }
    loginForm.addEventListener("submit", function (ev) {
      ev.preventDefault();
      var f = new FormData(loginForm);
      rawFetch("POST", "/auth/login", { email: f.get("email"), password: f.get("password") }).then(handle).catch(fail);
    });
    mfaForm.addEventListener("submit", function (ev) {
      ev.preventDefault();
      var f = new FormData(mfaForm);
      rawFetch("POST", "/auth/login/2fa", { mfa_token: mfaToken, code: f.get("code") }).then(handle).catch(fail);
    });
  }

  // ---------- dashboard ----------
  function initDashboard() {
    api("GET", "/admin/stats").then(function (s) {
      var box = $("#stats");
      [["Total users", s.users_total], ["Active", s.users_active], ["Verified", s.users_verified], ["2FA enabled", s.users_2fa]]
        .forEach(function (p) {
          box.appendChild(el("div", { class: "stat" }, [el("div", { class: "n", text: String(p[1]) }), el("div", { class: "l", text: p[0] })]));
        });
    }).catch(fail);
  }

  // ---------- pagination ----------
  function pager(load) {
    var state = { page: 1, perPage: 20, total: 0 };
    $("#prev").addEventListener("click", function () { if (state.page > 1) { state.page--; load(state); } });
    $("#next").addEventListener("click", function () { if (state.page * state.perPage < state.total) { state.page++; load(state); } });
    state.update = function (res) {
      state.total = res.total; state.page = res.page; state.perPage = res.per_page;
      var pages = Math.max(1, Math.ceil(res.total / res.per_page));
      $("#pageinfo").textContent = "Page " + res.page + " of " + pages + " (" + res.total + " total)";
      $("#prev").disabled = res.page <= 1;
      $("#next").disabled = res.page >= pages;
    };
    return state;
  }

  // ---------- users ----------
  function initUsers() {
    var q = "", me = JSON.parse(store.getItem("me"));
    var state = pager(load);

    function act(label, cls, fn) {
      var b = el("button", { type: "button", class: "btn " + cls, text: label });
      b.addEventListener("click", fn);
      return b;
    }
    function patch(u, body) {
      return api("PATCH", "/admin/users/" + encodeURIComponent(u.id), body).then(function () { msg("Saved", true); load(state); }).catch(fail);
    }
    function row(u) {
      var self = u.id === me.id;
      var actions = el("td", {});
      if (!self) {
        actions.appendChild(act(u.is_active ? "Disable" : "Enable", "btn-secondary", function () { patch(u, { is_active: !u.is_active }); }));
        actions.appendChild(act(u.role === "admin" ? "Make user" : "Make admin", "btn-secondary", function () { patch(u, { role: u.role === "admin" ? "user" : "admin" }); }));
        actions.appendChild(act("Revoke sessions", "btn-secondary", function () {
          api("POST", "/admin/users/" + encodeURIComponent(u.id) + "/revoke-sessions").then(function () { msg("Sessions revoked", true); }).catch(fail);
        }));
        actions.appendChild(act("Delete", "btn-danger", function () {
          if (window.confirm("Delete " + u.email + "? This cannot be undone.")) {
            api("DELETE", "/admin/users/" + encodeURIComponent(u.id)).then(function () { msg("Deleted", true); load(state); }).catch(fail);
          }
        }));
      } else {
        actions.appendChild(el("span", { class: "meta", text: "(you)" }));
      }
      var yn = function (v) { return v ? "yes" : "no"; };
      return el("tr", {}, [
        el("td", { text: u.email }), el("td", { text: u.name }), el("td", { text: u.role }),
        el("td", { text: yn(u.is_active) }), el("td", { text: yn(u.email_verified) }), el("td", { text: yn(u.totp_enabled) }),
        el("td", { text: String(u.passkey_count) }), actions,
      ]);
    }
    function load(s) {
      api("GET", "/admin/users?page=" + s.page + "&per_page=" + s.perPage + "&q=" + encodeURIComponent(q)).then(function (res) {
        var body = $("#rows");
        body.textContent = "";
        res.items.forEach(function (u) { body.appendChild(row(u)); });
        s.update(res);
      }).catch(fail);
    }
    $("#search-form").addEventListener("submit", function (ev) {
      ev.preventDefault();
      q = new FormData(ev.target).get("q") || "";
      state.page = 1;
      load(state);
    });
    load(state);
  }

  // ---------- audit ----------
  function initAudit() {
    var state = pager(load);
    function load(s) {
      api("GET", "/admin/audit-logs?page=" + s.page + "&per_page=" + s.perPage).then(function (res) {
        var body = $("#rows");
        body.textContent = "";
        res.items.forEach(function (l) {
          body.appendChild(el("tr", {}, [
            el("td", { text: new Date(l.created_at).toLocaleString() }), el("td", { text: l.action }),
            el("td", { class: "meta", text: l.user_id || "" }), el("td", { class: "meta", text: l.actor_id || "" }),
            el("td", { text: l.ip }), el("td", { class: "meta", text: JSON.stringify(l.metadata || {}) }),
          ]));
        });
        s.update(res);
      }).catch(fail);
    }
    load(state);
  }

  // ---------- boot ----------
  var page = document.body.getAttribute("data-page");
  if (page === "login") { initLogin(); return; }
  if (!initChrome()) return;
  if (page === "dashboard") initDashboard();
  else if (page === "users") initUsers();
  else if (page === "audit") initAudit();
})();
