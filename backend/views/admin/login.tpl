{{template "admin/partial_head.tpl" .}}
<main class="card narrow">
  <h1>Admin sign in</h1>
  <p id="msg" class="msg" role="alert"></p>
  <form id="login-form" autocomplete="on">
    <label>Email <input type="email" name="email" required autocomplete="username"></label>
    <label>Password <input type="password" name="password" required autocomplete="current-password"></label>
    <button type="submit" class="btn">Sign in</button>
  </form>
  <form id="mfa-form" class="hidden" autocomplete="one-time-code">
    <p>Enter your 6-digit authenticator code or a recovery code.</p>
    <label>Code <input type="text" name="code" required inputmode="text" autocomplete="one-time-code"></label>
    <button type="submit" class="btn">Verify</button>
  </form>
</main>
{{template "admin/partial_foot.tpl" .}}
