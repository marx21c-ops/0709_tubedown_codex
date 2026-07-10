package handler

import "github.com/gofiber/fiber/v2"

func Home() fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
		return c.SendString(homeHTML)
	}
}

const homeHTML = `<!doctype html>
<html lang="ko">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>TubeDown</title>
  <style>
    :root {
      color-scheme: light;
      --bg: #f6f7f9;
      --panel: #ffffff;
      --text: #15171a;
      --muted: #667085;
      --line: #d8dde5;
      --accent: #0f766e;
      --accent-dark: #115e59;
      --danger: #b42318;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      min-height: 100vh;
      background: var(--bg);
      color: var(--text);
      font: 15px/1.5 -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
    }
    main {
      width: min(920px, calc(100% - 32px));
      margin: 0 auto;
      padding: 56px 0;
    }
    header { margin-bottom: 28px; }
    h1 {
      margin: 0 0 8px;
      font-size: clamp(32px, 5vw, 52px);
      line-height: 1.05;
      letter-spacing: 0;
    }
    p { margin: 0; color: var(--muted); }
    .panel {
      background: var(--panel);
      border: 1px solid var(--line);
      border-radius: 8px;
      padding: 20px;
      box-shadow: 0 14px 40px rgba(16, 24, 40, 0.08);
    }
    form {
      display: grid;
      grid-template-columns: 1fr auto;
      gap: 12px;
    }
    input, button {
      height: 46px;
      border-radius: 6px;
      font: inherit;
    }
    input {
      width: 100%;
      border: 1px solid var(--line);
      padding: 0 14px;
      color: var(--text);
      background: #fff;
    }
    input:focus {
      outline: 2px solid rgba(15, 118, 110, 0.22);
      border-color: var(--accent);
    }
    button {
      border: 0;
      padding: 0 18px;
      background: var(--accent);
      color: #fff;
      cursor: pointer;
      font-weight: 650;
      white-space: nowrap;
    }
    button:hover { background: var(--accent-dark); }
    button:disabled { cursor: wait; opacity: 0.65; }
    .status {
      min-height: 24px;
      margin-top: 14px;
      color: var(--muted);
    }
    .status.error { color: var(--danger); }
    .result {
      display: none;
      margin-top: 18px;
      border-top: 1px solid var(--line);
      padding-top: 18px;
    }
    .meta {
      display: grid;
      grid-template-columns: 160px 1fr;
      gap: 16px;
      align-items: start;
      margin-bottom: 16px;
    }
    .thumb {
      width: 100%;
      aspect-ratio: 16 / 9;
      border-radius: 6px;
      object-fit: cover;
      background: #edf0f3;
    }
    h2 {
      margin: 0 0 6px;
      font-size: 20px;
      line-height: 1.25;
      letter-spacing: 0;
    }
    .formats {
      display: grid;
      gap: 8px;
    }
    .format {
      display: grid;
      grid-template-columns: 1fr auto;
      gap: 12px;
      align-items: center;
      min-height: 48px;
      padding: 10px 12px;
      border: 1px solid var(--line);
      border-radius: 6px;
    }
    .format small { color: var(--muted); }
    .format a {
      color: var(--accent);
      font-weight: 700;
      text-decoration: none;
      white-space: nowrap;
    }
    footer {
      margin-top: 18px;
      color: var(--muted);
      font-size: 13px;
    }
    @media (max-width: 640px) {
      main { width: min(100% - 24px, 920px); padding: 28px 0; }
      .panel { padding: 16px; }
      form, .meta, .format { grid-template-columns: 1fr; }
      button { width: 100%; }
    }
  </style>
</head>
<body>
  <main>
    <header>
      <h1>TubeDown</h1>
      <p>YouTube, TikTok, X, Instagram, Reddit, Twitch, SoundCloud 등 공개 URL의 메타데이터를 확인하세요.</p>
    </header>

    <section class="panel">
      <form id="lookup-form">
        <input id="url" name="url" type="url" placeholder="https://www.youtube.com/watch?v=..." autocomplete="off" required>
        <button id="submit" type="submit">조회</button>
      </form>
      <div id="status" class="status"></div>
      <div id="result" class="result">
        <div class="meta">
          <img id="thumbnail" class="thumb" alt="">
          <div>
            <h2 id="title"></h2>
            <p id="duration"></p>
          </div>
        </div>
        <div id="formats" class="formats"></div>
      </div>
    </section>

    <footer>개인 백업 또는 본인 콘텐츠 보관 목적에 맞게 사용하세요.</footer>
  </main>

  <script>
    const form = document.getElementById('lookup-form');
    const input = document.getElementById('url');
    const submit = document.getElementById('submit');
    const statusEl = document.getElementById('status');
    const result = document.getElementById('result');
    const title = document.getElementById('title');
    const duration = document.getElementById('duration');
    const thumbnail = document.getElementById('thumbnail');
    const formats = document.getElementById('formats');

    form.addEventListener('submit', async (event) => {
      event.preventDefault();
      const videoUrl = input.value.trim();
      if (!videoUrl) return;

      submit.disabled = true;
      result.style.display = 'none';
      formats.innerHTML = '';
      statusEl.className = 'status';
      statusEl.textContent = '메타데이터를 조회하는 중입니다.';

      try {
        const response = await fetch('/api/v1/metadata', {
          method: 'POST',
          headers: { 'content-type': 'application/json' },
          body: JSON.stringify({ url: videoUrl })
        });
        const data = await response.json();
        if (!response.ok) {
          throw new Error(data?.error?.message || '요청을 처리하지 못했습니다.');
        }

        title.textContent = data.title || '제목 없음';
        duration.textContent = data.duration ? Math.round(data.duration) + '초' : '';
        thumbnail.src = data.thumbnail || '';
        thumbnail.style.display = data.thumbnail ? 'block' : 'none';

        const visibleFormats = (data.formats || []).slice(0, 24);
        if (!visibleFormats.length) {
          formats.innerHTML = '<p>선택 가능한 형식이 없습니다.</p>';
        } else {
          for (const item of visibleFormats) {
            const row = document.createElement('div');
            row.className = 'format';
            const label = document.createElement('div');
            label.innerHTML = '<strong>' + escapeHTML(item.resolution || 'format') + '</strong><br><small>' + escapeHTML(item.ext || '') + ' · 영상+오디오 자동 병합</small>';
            const link = document.createElement('a');
            const params = new URLSearchParams({ url: videoUrl, format_id: item.format_id, title: data.title || 'download' });
            link.href = '/api/v1/download?' + params.toString();
            link.textContent = '다운로드';
            row.append(label, link);
            formats.append(row);
          }
        }

        result.style.display = 'block';
        statusEl.textContent = '조회가 완료됐습니다.';
      } catch (error) {
        statusEl.className = 'status error';
        statusEl.textContent = error.message;
      } finally {
        submit.disabled = false;
      }
    });

    function escapeHTML(value) {
      return String(value).replace(/[&<>"']/g, (char) => ({
        '&': '&amp;',
        '<': '&lt;',
        '>': '&gt;',
        '"': '&quot;',
        "'": '&#39;'
      }[char]));
    }
  </script>
</body>
</html>`
