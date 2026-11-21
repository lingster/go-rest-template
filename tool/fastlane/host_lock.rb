# Host-wide mutual exclusion between processes (separate fastlane runs on one Mac), used by
# ios/fastlane/Fastfile so two signing builds cannot interleave their keychain search-list changes.
# Pure Ruby: run the tests with `ruby tool/fastlane/host_lock_test.rb`.
#
# The lock is a directory taken with an atomic mkdir (works on macOS and Linux, no flock needed) holding
# the holder's PID. A holder that died without releasing is detected by its PID no longer being alive and
# the lock is removed.
require "fileutils"

module HostLock
  class TimeoutError < StandardError; end

  DEFAULT_POLL = 15
  DEFAULT_WAIT_MAX = 21_600 # 6 hours
  # A lock directory with no PID file yet is a holder still writing it; after this long it is abandoned.
  INIT_GRACE = 60

  module_function

  # Runs the block while holding the lock at dir, waiting for it if another process has it. Releases it
  # afterwards, also on an exception, but only if it is still ours. Raises TimeoutError after wait_max seconds.
  # pid is the process whose death frees the lock (default: this one). log is called with progress lines.
  def synchronize(dir, pid: Process.pid, poll: default(:CI_HOST_LOCK_POLL, DEFAULT_POLL),
                  wait_max: default(:CI_HOST_LOCK_WAIT_MAX, DEFAULT_WAIT_MAX), log: nil)
    acquire(dir, pid: pid, poll: poll, wait_max: wait_max, log: log)
    begin
      yield
    ensure
      release(dir, pid: pid)
    end
  end

  def acquire(dir, pid:, poll:, wait_max:, log: nil)
    FileUtils.mkdir_p(File.dirname(dir))
    started = now
    loop do
      return if try_acquire(dir, pid)
      next if stale?(dir) && steal(dir, log)

      waited = now - started
      holder = "pid #{holder_pid(dir) || '?'}"
      raise TimeoutError, "gave up after #{waited.round}s waiting for #{dir} held by #{holder}" if waited >= wait_max

      log&.call("waiting for #{dir} held by #{holder}; waited #{waited.round}s, next check in #{poll}s")
      sleep poll
    end
  end

  # Removes the lock if pid still owns it; a lock someone else took after ours was removed is left alone.
  def release(dir, pid:)
    FileUtils.rm_rf(dir) if holder_pid(dir) == pid
  end

  def try_acquire(dir, pid)
    Dir.mkdir(dir)
    write_atomically(File.join(dir, "pid"), pid.to_s)
    true
  rescue Errno::EEXIST
    false
  end

  # True if the lock directory exists but nobody holds it: its PID is not alive, or it never got a PID.
  def stale?(dir)
    return false unless File.directory?(dir)

    pid = holder_pid(dir)
    return !alive?(pid) if pid

    File.exist?(File.join(dir, "pid")) || (now_wall - mtime(dir)) > INIT_GRACE
  end

  # Removes a stale lock. One waiter at a time (a second mkdir lock, "<dir>.steal"), re-checking staleness
  # while holding it, so two waiters cannot remove a lock a third has just taken. True if it held the guard.
  def steal(dir, log)
    guard = "#{dir}.steal"
    begin
      Dir.mkdir(guard)
    rescue Errno::EEXIST
      # A stealer that died while holding the guard must not block everyone forever.
      FileUtils.rmdir(guard) if (now_wall - mtime(guard)) > INIT_GRACE
      return false
    end
    begin
      if stale?(dir)
        log&.call("removing stale lock #{dir} (pid #{holder_pid(dir) || '?'} is gone)")
        FileUtils.rm_rf(dir)
      end
    ensure
      FileUtils.rmdir(guard)
    end
    true
  end

  def holder_pid(dir)
    Integer(File.read(File.join(dir, "pid")).strip, 10)
  rescue Errno::ENOENT, ArgumentError
    nil
  end

  # EPERM means the process exists but belongs to another user: alive.
  def alive?(pid)
    Process.kill(0, pid)
    true
  rescue Errno::ESRCH
    false
  rescue Errno::EPERM
    true
  end

  def write_atomically(path, content)
    tmp = "#{path}.#{Process.pid}.tmp"
    File.write(tmp, content)
    File.rename(tmp, path)
  end

  def mtime(path)
    File.mtime(path).to_f
  rescue Errno::ENOENT
    now_wall # vanished meanwhile: treat as brand new
  end

  def now
    Process.clock_gettime(Process::CLOCK_MONOTONIC)
  end

  def now_wall
    Time.now.to_f
  end

  def default(env_name, fallback)
    value = ENV[env_name.to_s]
    value.nil? || value.empty? ? fallback : Float(value)
  end
end
