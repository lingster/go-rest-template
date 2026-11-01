{{template "admin/partial_head.tpl" .}}
{{template "admin/partial_nav.tpl" .}}
<main class="container">
  <h1>Users</h1>
  <p id="msg" class="msg" role="alert"></p>
  <form id="search-form" class="toolbar">
    <input type="search" name="q" placeholder="Search email or name">
    <button type="submit" class="btn">Search</button>
  </form>
  <table class="table">
    <thead><tr><th>Email</th><th>Name</th><th>Role</th><th>Active</th><th>Verified</th><th>2FA</th><th>Passkeys</th><th>Actions</th></tr></thead>
    <tbody id="rows"></tbody>
  </table>
  <div class="pager"><button type="button" id="prev" class="btn btn-secondary">Previous</button> <span id="pageinfo"></span> <button type="button" id="next" class="btn btn-secondary">Next</button></div>
</main>
{{template "admin/partial_foot.tpl" .}}
