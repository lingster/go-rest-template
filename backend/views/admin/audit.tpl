{{template "admin/partial_head.tpl" .}}
{{template "admin/partial_nav.tpl" .}}
<main class="container">
  <h1>Audit log</h1>
  <p id="msg" class="msg" role="alert"></p>
  <table class="table">
    <thead><tr><th>Time</th><th>Action</th><th>User</th><th>Actor</th><th>IP</th><th>Details</th></tr></thead>
    <tbody id="rows"></tbody>
  </table>
  <div class="pager"><button type="button" id="prev" class="btn btn-secondary">Previous</button> <span id="pageinfo"></span> <button type="button" id="next" class="btn btn-secondary">Next</button></div>
</main>
{{template "admin/partial_foot.tpl" .}}
