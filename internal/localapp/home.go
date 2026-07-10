package localapp

const localHTML = `<!doctype html>
<html lang="ko">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta name="tubedown-token" content="{{TOKEN}}">
  <title>TubeDown Local</title>
  <style nonce="{{NONCE}}">
    :root { color-scheme: light; --bg:#f4f6f8; --panel:#fff; --text:#15171a; --muted:#667085; --line:#d8dde5; --accent:#0f766e; --accent2:#115e59; --danger:#b42318; }
    * { box-sizing:border-box; }
    body { margin:0; min-height:100vh; background:var(--bg); color:var(--text); font:15px/1.5 -apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif; }
    main { width:min(900px,calc(100% - 28px)); margin:0 auto; padding:48px 0; }
    header { margin-bottom:24px; }
    h1 { margin:0; font-size:clamp(34px,6vw,56px); line-height:1; letter-spacing:-2px; }
    header p,.muted { color:var(--muted); }
    .local { display:inline-flex; margin-top:12px; padding:5px 9px; border-radius:999px; background:#dff7f1; color:#0b625b; font-size:12px; font-weight:700; }
    .panel { background:var(--panel); border:1px solid var(--line); border-radius:12px; padding:20px; box-shadow:0 16px 45px rgba(16,24,40,.08); }
    form { display:grid; grid-template-columns:1fr auto; gap:10px; }
    input,button,select { min-height:46px; border-radius:8px; font:inherit; }
    input,select { border:1px solid var(--line); background:#fff; padding:0 13px; }
    button { border:0; padding:0 18px; background:var(--accent); color:#fff; font-weight:700; cursor:pointer; }
    button:hover { background:var(--accent2); }
    button:disabled { opacity:.5; cursor:not-allowed; }
    .status { min-height:24px; margin-top:13px; color:var(--muted); }
    .status.error { color:var(--danger); }
    .result { display:none; margin-top:18px; padding-top:18px; border-top:1px solid var(--line); }
    .meta { display:grid; grid-template-columns:180px 1fr; gap:16px; align-items:start; }
    .meta img { width:100%; aspect-ratio:16/9; object-fit:cover; border-radius:8px; background:#edf0f3; }
    h2 { margin:0 0 10px; font-size:20px; line-height:1.3; }
    .controls { display:flex; flex-wrap:wrap; gap:8px; align-items:center; }
    .controls select { min-width:130px; }
    .cancel { background:#475467; }
    .job { display:none; margin-top:18px; padding:16px; border-radius:10px; background:#f8fafb; border:1px solid var(--line); }
    .bar { height:10px; margin:12px 0 8px; overflow:hidden; border-radius:999px; background:#e4e7ec; }
    .bar span { display:block; width:0; height:100%; background:var(--accent); transition:width .25s; }
    footer { margin-top:16px; color:var(--muted); font-size:13px; }
    @media (max-width:640px) { main{padding:26px 0}.panel{padding:15px}form,.meta{grid-template-columns:1fr}button{width:100%}.controls{display:grid}.controls select{width:100%} }
  </style>
</head>
<body>
<main>
  <header>
    <h1>TubeDown</h1>
    <p>YouTube 영상을 Mac으로 직접 다운로드하고 로컬에서 병합합니다.</p>
    <span class="local">LOCAL ONLY · 127.0.0.1</span>
  </header>
  <section class="panel">
    <form id="lookup">
      <input id="url" type="url" placeholder="https://www.youtube.com/watch?v=..." autocomplete="off" required>
      <button id="lookup-button" type="submit">조회</button>
    </form>
    <div id="status" class="status"></div>
    <div id="result" class="result">
      <div class="meta">
        <img id="thumbnail" alt="">
        <div>
          <h2 id="title"></h2>
          <div class="controls">
            <select id="quality" aria-label="화질"></select>
            <button id="download" type="button">Mac으로 다운로드</button>
          </div>
        </div>
      </div>
    </div>
    <div id="job" class="job">
      <strong id="job-label">다운로드 준비 중</strong>
      <div class="bar"><span id="progress"></span></div>
      <div class="controls">
        <span id="percent" class="muted">0%</span>
        <button id="cancel" class="cancel" type="button">취소</button>
      </div>
    </div>
  </section>
  <footer>저장 위치: {{DOWNLOAD_DIR}} · 영상과 오디오는 이 Mac에서만 처리됩니다.</footer>
</main>
<script nonce="{{NONCE}}">
  const token = document.querySelector('meta[name="tubedown-token"]').content;
  const headers = {'content-type':'application/json','X-TubeDown-Token':token};
  const lookup = document.getElementById('lookup');
  const urlInput = document.getElementById('url');
  const lookupButton = document.getElementById('lookup-button');
  const statusEl = document.getElementById('status');
  const result = document.getElementById('result');
  const titleEl = document.getElementById('title');
  const thumbnail = document.getElementById('thumbnail');
  const quality = document.getElementById('quality');
  const downloadButton = document.getElementById('download');
  const jobEl = document.getElementById('job');
  const jobLabel = document.getElementById('job-label');
  const progress = document.getElementById('progress');
  const percent = document.getElementById('percent');
  const cancelButton = document.getElementById('cancel');
  let metadata = null;
  let pollTimer = null;

  async function api(path, options={}) {
    const response = await fetch(path, {...options, headers:{...headers,...(options.headers||{})}});
    if (response.status === 204) return null;
    const data = await response.json();
    if (!response.ok) throw new Error(data?.error?.message || '요청을 처리하지 못했습니다.');
    return data;
  }

  lookup.addEventListener('submit', async (event) => {
    event.preventDefault();
    lookupButton.disabled = true;
    result.style.display = 'none';
    statusEl.className = 'status';
    statusEl.textContent = '영상 정보를 확인하고 있습니다.';
    try {
      metadata = await api('/api/metadata',{method:'POST',body:JSON.stringify({url:urlInput.value.trim()})});
      titleEl.textContent = metadata.title || '제목 없음';
      thumbnail.src = metadata.thumbnail || '';
      quality.replaceChildren(...(metadata.formats||[]).map(item => {
        const option = document.createElement('option');
        option.value = item.format_id;
        option.textContent = item.resolution + ' MP4';
        return option;
      }));
      result.style.display = 'block';
      statusEl.textContent = '화질을 선택하고 다운로드하세요.';
    } catch (error) {
      statusEl.className = 'status error';
      statusEl.textContent = error.message;
    } finally { lookupButton.disabled = false; }
  });

  downloadButton.addEventListener('click', async () => {
    if (!metadata || !quality.value) return;
    downloadButton.disabled = true;
    try {
      await api('/api/download',{method:'POST',body:JSON.stringify({url:urlInput.value.trim(),title:metadata.title,format_id:quality.value})});
      jobEl.style.display = 'block';
      statusEl.textContent = 'YouTube에서 이 Mac으로 직접 다운로드 중입니다.';
      poll();
    } catch (error) {
      statusEl.className = 'status error';
      statusEl.textContent = error.message;
      downloadButton.disabled = false;
    }
  });

  async function poll() {
    clearTimeout(pollTimer);
    try {
      const job = await api('/api/job');
      const value = Math.max(0,Math.min(100,job.progress||0));
      progress.style.width = value + '%';
      percent.textContent = value.toFixed(1) + '%';
      jobLabel.textContent = job.status === 'queued' ? '준비 중' : job.status === 'downloading' ? '다운로드 및 병합 중' : job.status === 'complete' ? '완료' : job.status === 'cancelled' ? '취소됨' : '실패';
      if (job.status === 'complete') {
        statusEl.className = 'status';
        statusEl.textContent = '다운로드가 완료됐습니다. Finder의 TubeDown 폴더를 확인하세요.';
        downloadButton.disabled = false;
        cancelButton.disabled = true;
        return;
      }
      if (job.status === 'failed' || job.status === 'cancelled') {
        statusEl.className = 'status error';
        statusEl.textContent = job.error || '다운로드가 중단됐습니다.';
        downloadButton.disabled = false;
        cancelButton.disabled = true;
        return;
      }
      pollTimer = setTimeout(poll,1000);
    } catch (error) {
      statusEl.className = 'status error';
      statusEl.textContent = error.message;
      downloadButton.disabled = false;
    }
  }

  cancelButton.addEventListener('click', async () => {
    cancelButton.disabled = true;
    try { await api('/api/cancel',{method:'POST',body:'{}'}); poll(); }
    catch (error) { statusEl.className='status error'; statusEl.textContent=error.message; cancelButton.disabled=false; }
  });
</script>
</body>
</html>`
