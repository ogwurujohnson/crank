(function () {
  "use strict";

  const $ = (sel, el = document) => el.querySelector(sel);
  const $$ = (sel, el = document) => [...el.querySelectorAll(sel)];

  const routes = ["dashboard", "pending", "retries", "dead", "registry", "settings"];

  function api(path) {
    const base = document.querySelector("base")?.href || "/";
    const u = new URL(path.replace(/^\//, ""), base);
    return u.toString();
  }

  async function fetchJSON(path) {
    const r = await fetch(api(path), { headers: { Accept: "application/json" } });
    if (!r.ok) throw new Error((await r.text()) || r.statusText);
    return r.json();
  }

  function esc(s) {
    if (s == null) return "";
    const d = document.createElement("div");
    d.textContent = String(s);
    return d.innerHTML;
  }

  function formatTime(ts) {
    if (ts == null || ts === 0) return "—";
    const d = new Date(typeof ts === "number" && ts < 1e12 ? ts * 1000 : ts);
    if (Number.isNaN(d.getTime())) return "—";
    return d.toLocaleTimeString(undefined, { hour12: false, hour: "2-digit", minute: "2-digit", second: "2-digit" }) + "." + String(d.getMilliseconds()).padStart(3, "0");
  }

  function shortJID(jid) {
    if (!jid || jid.length < 12) return jid || "—";
    return "#" + jid.slice(0, 4) + "…" + jid.slice(-6);
  }

  function priorityBars(meta) {
    const p = meta && meta.priority;
    const urgent = p === "urgent" || p === "URGENT";
    const n = urgent ? 3 : p && /^\d+$/.test(String(p)) ? Math.min(3, Math.max(1, parseInt(p, 10))) : 2;
    let html = '<span class="priority-pill' + (urgent ? " urgent" : "") + '">';
    for (let i = 0; i < 3; i++) html += "<span" + (i < n ? ' class="on"' : "") + "></span>";
    html += "</span>";
    const label = urgent ? "URGENT" : "P-" + String(n).padStart(2, "0");
    return html + " " + esc(label);
  }

  function sparkSVG() {
    return (
      '<svg class="spark" viewBox="0 0 36 14" aria-hidden="true">' +
      '<path d="M0 10 L6 4 L12 11 L18 2 L24 9 L30 5 L36 8" fill="none" stroke="currentColor" stroke-width="1" opacity="0.5"/>' +
      "</svg>"
    );
  }

  let meta = { version: "—", project: "—", title: "THE ENGINEERING EDITORIAL" };
  let stats = null;
  let pollTimer = null;

  function setActiveNav(route) {
    $$(".top-nav a").forEach((a) => {
      const tr = a.dataset.route;
      let active = tr === route;
      if (tr === "pending" && ["pending", "retries", "dead"].includes(route)) active = true;
      a.classList.toggle("is-active", active);
    });
    $$(".side-nav a").forEach((a) => a.classList.toggle("is-active", a.dataset.route === route));
  }

  function shellHTML() {
    return (
      '<header class="top-bar">' +
      '<div class="brand">' +
      esc(meta.title) +
      "</div>" +
      '<nav class="top-nav">' +
      '<a href="#/dashboard" data-route="dashboard">Analytics</a>' +
      '<a href="#/pending" data-route="pending">Queue</a>' +
      '<a href="#/registry" data-route="registry">Registry</a>' +
      "</nav>" +
      '<div class="top-search hidden">' +
      '<div class="top-search-wrap"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><path d="M21 21l-4.35-4.35"/></svg>' +
      '<input type="search" placeholder="QUERY JOBS..." aria-label="Query jobs" /></div></div>' +
      '<div class="top-actions">' +
      '<button type="button" class="icon-btn" aria-label="Notifications"><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9"/><path d="M13.73 21a2 2 0 0 1-3.46 0"/></svg></button>' +
      '<button type="button" class="icon-btn" aria-label="Terminal"><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M8 9l-4 4 4 4M16 9l4 4-4 4M13 5l-2 14"/></svg></button>' +
      '<span class="avatar" aria-hidden="true">CR</span>' +
      "</div></header>" +
      '<div class="shell">' +
      '<aside class="sidebar">' +
      '<div class="sidebar-meta"><svg class="grid-icon" width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M3 3h7v7H3V3zm11 0h7v7h-7V3zM3 14h7v7H3v-7zm11 0h7v7h-7v-7z"/></svg>' +
      "<div><div class=\"project\">" +
      esc(meta.project) +
      '</div><div class="ver">' +
      esc(meta.version) +
      "</div></div></div>" +
      '<nav class="side-nav">' +
      '<a href="#/dashboard" data-route="dashboard"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><rect x="3" y="3" width="7" height="7"/><rect x="14" y="3" width="7" height="7"/><rect x="3" y="14" width="7" height="7"/><rect x="14" y="14" width="7" height="7"/></svg><span>Dashboard</span></a>' +
      '<a href="#/pending" data-route="pending"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M12 6v6l4 2"/><circle cx="12" cy="12" r="10"/></svg><span>Pending</span></a>' +
      '<a href="#/retries" data-route="retries"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M23 4v6h-6M1 20v-6h6"/><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/></svg><span>Retries</span></a>' +
      '<a href="#/dead" data-route="dead"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><circle cx="12" cy="12" r="10"/><path d="M15 9l-6 6M9 9l6 6"/></svg><span>Dead jobs</span></a>' +
      '<a href="#/settings" data-route="settings"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><circle cx="12" cy="12" r="3"/><path d="M12 1v2M12 21v2M4.22 4.22l1.42 1.42M18.36 18.36l1.42 1.42M1 12h2M21 12h2M4.22 19.78l1.42-1.42M18.36 5.64l1.42-1.42"/></svg><span>Settings</span></a>' +
      "</nav>" +
      '<button type="button" class="btn-run-job" id="btn-run-demo"><svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor"><path d="M13 2L3 14h8l-1 8 10-12h-8l1-8z"/></svg><span>Run job</span></button>' +
      '<div class="sidebar-foot">' +
      '<a href="https://github.com/ogwurujohnson/crank" target="_blank" rel="noopener">Documentation</a>' +
      "<a href=\"#\">Support</a>" +
      "</div></aside>" +
      '<main class="main" id="main-outlet"></main></div>'
    );
  }

  async function renderDashboard(outlet) {
    outlet.innerHTML = '<div class="page-head"><div class="page-title-block"><h1>Analytics <span class="badge-live"><span class="dot"></span>LIVE</span></h1><p class="sub">Queue depth, throughput signals, and worker pool posture at a glance.</p></div></div><div class="dash-grid" id="dash-tiles"><p class="loading">Loading…</p></div>';
    try {
      stats = await fetchJSON("api/stats");
      const q = stats.queues || {};
      const queueRows = Object.keys(q)
        .sort()
        .map((name) => ({ name, n: q[name] }));
      let html = "";
      html +=
        '<div class="stat-tile"><div class="label">Processed (stat set)</div><div class="value">' +
        esc(stats.processed) +
        "</div></div>";
      html += '<div class="stat-tile"><div class="label">Retry backlog</div><div class="value">' + esc(stats.retry) + "</div></div>";
      html += '<div class="stat-tile"><div class="label">Dead letter</div><div class="value">' + esc(stats.dead) + "</div></div>";
      for (const row of queueRows) {
        html +=
          '<div class="stat-tile"><div class="label">Queue · ' +
          esc(row.name) +
          '</div><div class="value">' +
          esc(row.n) +
          "</div></div>";
      }
      $("#dash-tiles", outlet).innerHTML = html;
    } catch (e) {
      $("#dash-tiles", outlet).innerHTML = '<p class="empty-hint">' + esc(e.message) + "</p>";
    }
  }

  async function renderPending(outlet) {
    const pendingCount =
      stats && stats.queues ? Object.values(stats.queues).reduce((a, b) => a + Number(b || 0), 0) : null;
    const pendingBadge =
      pendingCount == null ? "—" : "[" + String(pendingCount).padStart(3, "0") + "]";
    outlet.innerHTML =
      '<div class="page-head">' +
      '<div class="page-title-block"><h1>Pending queue <span class="badge-live" style="font-family:var(--font-mono)">LIVE ' +
      esc(pendingBadge) +
      '</span></h1><p class="sub">Weighted worker pool polling your configured queues. Jobs listed in dequeue order (next run first).</p></div>' +
      '<div class="head-actions">' +
      '<button type="button" class="btn-ghost" id="btn-flush" title="No-op in read-only UI">Flush cache</button>' +
      "</div></div>" +
      '<div class="filter-bar">' +
      '<div class="filter-search"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><path d="M21 21l-4.35-4.35"/></svg>' +
      '<input type="search" id="pending-filter" placeholder="Filter by Job ID, Type, or Metadata…" /></div>' +
      '<select class="filter-select" id="pending-queue"><option value="">All types</option></select>' +
      '<button type="button" class="icon-btn" aria-label="Filter settings"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><circle cx="12" cy="12" r="3"/><path d="M12 1v2M12 21v2"/></svg></button>' +
      '<span class="bulk-label">Bulk actions</span>' +
      '<button type="button" class="btn-ghost btn-danger-ghost" disabled>Terminate</button>' +
      '<button type="button" class="btn-ghost btn-success-ghost" disabled>Boost</button>' +
      "</div>" +
      '<div class="table-wrap"><div class="table-scroll"><table class="data-table"><thead><tr>' +
      "<th>State</th><th>Job identifier</th><th>Process class</th><th>Inbound</th><th>Priority</th><th>Telemetry</th>" +
      '</tr></thead><tbody id="pending-body"></tbody></table></div>' +
      '<div class="table-footer"><span id="pending-foot-left">Showing —</span><div class="pager" id="pending-pager"></div></div></div>';

    const sel = $("#pending-queue", outlet);
    const names = stats && stats.queues ? Object.keys(stats.queues).sort() : [];
    for (const n of names) {
      const o = document.createElement("option");
      o.value = n;
      o.textContent = n;
      sel.appendChild(o);
    }

    let rows = [];
    try {
      rows = await fetchJSON("api/pending?limit=100");
    } catch (e) {
      $("#pending-body", outlet).innerHTML = '<tr><td colspan="6"><p class="empty-hint">' + esc(e.message) + "</p></td></tr>";
      return;
    }

    function applyFilter() {
      const q = $("#pending-filter", outlet).value.toLowerCase();
      const qq = sel.value;
      return rows.filter((r) => {
        if (qq && r.queue !== qq) return false;
        if (!q) return true;
        const blob = (r.job.jid + " " + r.job.class + " " + JSON.stringify(r.job.metadata || {})).toLowerCase();
        return blob.includes(q);
      });
    }

    const pageSize = 10;
    let page = 1;

    function paint() {
      const filtered = applyFilter();
      const total = filtered.length;
      const pages = Math.max(1, Math.ceil(total / pageSize));
      if (page > pages) page = pages;
      const start = (page - 1) * pageSize;
      const slice = filtered.slice(start, start + pageSize);
      const tb = $("#pending-body", outlet);
      if (slice.length === 0) {
        tb.innerHTML = '<tr><td colspan="6"><p class="empty-hint">No matching jobs</p></td></tr>';
      } else {
        tb.innerHTML = slice
          .map((r) => {
            const j = r.job;
            const ts = j.enqueued_at || j.created_at;
            return (
              "<tr><td><div class=\"row-flex\"><span class=\"state-bar pending\"></span></div></td>" +
              '<td class="jid">' +
              esc(shortJID(j.jid)) +
              '</td><td class="cell-class"><span class="tag-class">' +
              esc(j.class) +
              '</span></td><td>' +
              esc(formatTime(ts)) +
              "</td><td>" +
              priorityBars(j.metadata) +
              "</td><td>" +
              sparkSVG() +
              "</td></tr>"
            );
          })
          .join("");
      }
      $("#pending-foot-left", outlet).innerHTML =
        "Showing " +
        (total ? start + 1 : 0) +
        "-" +
        Math.min(start + pageSize, total) +
        " of " +
        total +
        ' records <span class="socket-live"><span class="dot"></span>Socket connected</span>';
      const pg = $("#pending-pager", outlet);
      pg.innerHTML = "";
      const mk = (label, dis, onClick, active) => {
        const b = document.createElement("button");
        b.textContent = label;
        b.disabled = dis;
        if (active) b.classList.add("active");
        b.addEventListener("click", onClick);
        pg.appendChild(b);
      };
      mk("<", page <= 1, () => {
        page--;
        paint();
      });
      for (let i = 1; i <= pages && i <= 5; i++) {
        mk(String(i), false, () => {
          page = i;
          paint();
        }, i === page);
      }
      mk(">", page >= pages, () => {
        page++;
        paint();
      });
    }

    $("#pending-filter", outlet).addEventListener("input", () => {
      page = 1;
      paint();
    });
    sel.addEventListener("change", () => {
      page = 1;
      paint();
    });
    paint();
  }

  async function renderRetries(outlet) {
    outlet.innerHTML =
      '<div class="page-head"><div class="page-title-block"><h1>Retries <span class="badge-live mono">[' +
      esc(stats ? String(stats.retry).padStart(3, "0") : "—") +
      ']</span></h1><p class="sub">Jobs scheduled for re-enqueue after backoff.</p></div><div class="head-actions"><button type="button" class="btn-ghost" disabled>Delete dead jobs</button><button type="button" class="btn-primary-solid" disabled>Retry all</button></div></div>' +
      '<div class="split"><div id="retry-list"></div><div class="terminal-panel" id="retry-detail"><div class="terminal-title"><span><span class="terminal-dots"><i></i><i></i><i></i></span>Stacktrace preview</span><span></span></div><div class="terminal-body"><p class="empty-hint">Select a job</p></div></div></div>';

    let list = [];
    try {
      list = await fetchJSON("api/retries?limit=50");
    } catch (e) {
      $("#retry-list", outlet).innerHTML = '<p class="empty-hint">' + esc(e.message) + "</p>";
      return;
    }

    const holder = $("#retry-list", outlet);
    let selected = 0;

    function detail(rs) {
      const panel = $(".terminal-body", $("#retry-detail", outlet));
      if (!rs) {
        panel.innerHTML = '<p class="empty-hint">Select a job</p>';
        return;
      }
      const j = rs.job;
      const when = rs.retry_at ? new Date(rs.retry_at).toLocaleString() : "—";
      panel.innerHTML =
        '<div class="exception">RETRY_SCHEDULED · next run ' +
        esc(when) +
        "</div>" +
        '<div class="payload-box">' +
        esc(JSON.stringify({ jid: j.jid, class: j.class, queue: j.queue, retry_count: j.retry_count, retry_max: j.retry, args: j.args }, null, 2)) +
        "</div>" +
        '<div class="log-line info"><span class="lvl">[INFO]</span>Job held in retry sorted set until score ≤ now.</div>' +
        '<div class="log-line warn"><span class="lvl">[WARN]</span>Inspect worker logs on the consumer for last failure reason.</div>';
    }

    if (list.length === 0) {
      holder.innerHTML = '<p class="empty-hint">No scheduled retries</p>';
      return;
    }

    holder.innerHTML = list
      .map((rs, i) => {
        const j = rs.job;
        return (
          '<div class="job-card' +
          (i === 0 ? " is-selected" : "") +
          '" data-i="' +
          i +
          '"><div class="row-top"><span>ID: ' +
          esc(j.jid.slice(0, 12).toUpperCase() + "…") +
          '</span><span>RETRY [' +
          esc(j.retry_count) +
          "/" +
          esc(j.retry) +
          "]</span></div>" +
          '<div class="class-name">' +
          esc(j.class) +
          '</div><div class="err-msg">Scheduled re-enqueue from Crank retry maintenance loop.</div>' +
          '<div class="meta">' +
          esc(j.queue) +
          " · backoff tier " +
          esc(j.retry_count) +
          "</div></div>"
        );
      })
      .join("");

    $$(".job-card", holder).forEach((card) => {
      card.addEventListener("click", () => {
        $$(".job-card", holder).forEach((c) => c.classList.remove("is-selected"));
        card.classList.add("is-selected");
        selected = +card.dataset.i;
        detail(list[selected]);
      });
    });
    detail(list[0]);
  }

  async function renderDead(outlet) {
    outlet.innerHTML =
      '<div class="page-head"><div class="page-title-block"><h1>Job exceptions <span class="badge-live mono">[' +
      esc(stats ? String(stats.dead).padStart(3, "0") : "—") +
      ']</span></h1><p class="sub">Monitoring retry-exhausted and terminal job states.</p></div><div class="head-actions"><button type="button" class="btn-ghost" disabled>Delete dead jobs</button><button type="button" class="btn-primary-solid" disabled>Retry all</button></div></div>' +
      '<div class="split"><div id="dead-list"></div><div class="terminal-panel" id="dead-detail"><div class="terminal-title"><span><span class="terminal-dots"><i></i><i></i><i></i></span>Stacktrace preview</span><span><button type="button" class="btn-ghost" style="padding:0.2rem 0.45rem;font-size:0.58rem">Copy</button></span></div><div class="terminal-body"><p class="empty-hint">Select a job</p></div><div class="terminal-actions"><button type="button" class="btn-ghost" disabled>Ignore failure</button><button type="button" class="btn-primary-solid" disabled>Force re-queue</button></div></div></div>';

    let jobs = [];
    try {
      jobs = await fetchJSON("api/dead?limit=50");
    } catch (e) {
      $("#dead-list", outlet).innerHTML = '<p class="empty-hint">' + esc(e.message) + "</p>";
      return;
    }

    const holder = $("#dead-list", outlet);

    function detail(j) {
      const panel = $(".terminal-body", $("#dead-detail", outlet));
      if (!j) {
        panel.innerHTML = '<p class="empty-hint">Select a job</p>';
        return;
      }
      const errLine =
        "Job moved to dead letter after exceeding retry budget. Class " + j.class + ", jid " + j.jid + ".";
      panel.innerHTML =
        '<div class="exception">' +
        esc(errLine) +
        "</div>" +
        '<div class="payload-box">' +
        esc(JSON.stringify({ jid: j.jid, class: j.class, queue: j.queue, retry_count: j.retry_count, args: j.args, metadata: j.metadata }, null, 2)) +
        "</div>" +
        '<div class="log-line info"><span class="lvl">[INFO]</span>Payload snapshot from broker dead set (read-only).</div>' +
        '<div class="log-line err"><span class="lvl">[ERROR]</span>Terminal failure — inspect application logs for exception chain.</div>';
    }

    if (jobs.length === 0) {
      holder.innerHTML = '<p class="empty-hint">No dead jobs</p>';
      return;
    }

    holder.innerHTML = jobs
      .map((j, i) => {
        return (
          '<div class="job-card' +
          (i === 0 ? " is-selected" : "") +
          '" data-i="' +
          i +
          '"><div class="row-top"><span>ID: ' +
          esc(j.jid.slice(0, 8).toUpperCase() + "_DL") +
          '</span><span>DEAD</span></div>' +
          '<div class="class-name">' +
          esc(j.class) +
          '</div><div class="err-msg">MAX_RETRIES_EXCEEDED · moved to dead queue after ' +
          esc(j.retry) +
          " attempts.</div>" +
          '<div class="meta">' +
          esc(formatTime(j.enqueued_at || j.created_at)) +
          " · consumer process</div></div>"
        );
      })
      .join("");

    $$(".job-card", holder).forEach((card) => {
      card.addEventListener("click", () => {
        $$(".job-card", holder).forEach((c) => c.classList.remove("is-selected"));
        card.classList.add("is-selected");
        detail(jobs[+card.dataset.i]);
      });
    });
    detail(jobs[0]);
  }

  async function renderRegistry(outlet) {
    outlet.innerHTML =
      '<div class="page-head"><div class="page-title-block"><h1>Registry</h1><p class="sub">Worker classes registered on this engine (local registry and global RegisterWorker).</p></div></div><div class="registry-list" id="reg-out"><p class="loading">Loading…</p></div>';
    try {
      const data = await fetchJSON("api/registry");
      const w = data.workers || [];
      $("#reg-out", outlet).innerHTML = w.length ? w.map((n) => '<div class="registry-item">' + esc(n) + "</div>").join("") : '<p class="empty-hint">No workers registered</p>';
    } catch (e) {
      $("#reg-out", outlet).innerHTML = '<p class="empty-hint">' + esc(e.message) + "</p>";
    }
  }

  function renderSettings(outlet) {
    outlet.innerHTML =
      '<div class="page-head"><div class="page-title-block"><h1>Settings</h1><p class="sub">Dashboard is read-only; actions are wired for future operator APIs.</p></div></div>' +
      '<div class="settings-block"><h3>Consumer</h3><p>Mount <code style="color:var(--secondary)">DashboardHandler</code> on your HTTP server or call <code style="color:var(--secondary)">StartDashboard</code> with a dedicated addr.</p></div>';
  }

  async function route() {
    const h = (location.hash || "#/dashboard").replace(/^#\/?/, "") || "dashboard";
    const route = h.split("/")[0] || "dashboard";
    const r = routes.includes(route) ? route : "dashboard";

    const app = $("#app");
    if (!$(".shell", app)) {
      app.innerHTML = shellHTML();
      $("#btn-run-demo", app).addEventListener("click", () => alert("Enqueue jobs from your application via the Crank client API."));
    }

    setActiveNav(r);

    const outlet = $("#main-outlet", app);
    if (r === "dashboard") await renderDashboard(outlet);
    else if (r === "pending") {
      try {
        stats = await fetchJSON("api/stats");
      } catch (_) {}
      await renderPending(outlet);
    } else if (r === "retries") {
      try {
        stats = await fetchJSON("api/stats");
      } catch (_) {}
      await renderRetries(outlet);
    } else if (r === "dead") {
      try {
        stats = await fetchJSON("api/stats");
      } catch (_) {}
      await renderDead(outlet);
    } else if (r === "registry") await renderRegistry(outlet);
    else renderSettings(outlet);
  }

  async function boot() {
    try {
      meta = await fetchJSON("api/meta");
    } catch (_) {}
    window.addEventListener("hashchange", route);
    await route();
    if (pollTimer) clearInterval(pollTimer);
    pollTimer = setInterval(async () => {
      try {
        stats = await fetchJSON("api/stats");
      } catch (_) {}
    }, 4000);
  }

  boot();
})();
