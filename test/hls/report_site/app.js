let chartInstances = [];

async function loadReport() {
  const res = await fetch('data/report.json');
  if (!res.ok) {
    document.getElementById('meta').textContent = 'No results yet — run: make integration-tests';
    return;
  }
  const report = await res.json();
  renderReport(report);
}

function card(label, value, cls) {
  return `<div class="card ${cls || ''}"><div class="card-label">${label}</div><div class="card-value">${value}</div></div>`;
}

function gpuPercent(row) {
  const v = row.resources?.gpuPercentAvg ?? row.hw?.gpuUtilAvg;
  return v != null && !Number.isNaN(v) ? v : null;
}

function gpuMonitor(row) {
  return row.resources?.gpuMonitor || row.hw?.gpuMonitor || '';
}

function gpuLabel(row) {
  const avg = gpuPercent(row);
  const mon = gpuMonitor(row);
  if (avg != null) {
    return { text: `${avg.toFixed(0)}%`, title: mon ? `Monitor: ${mon}` : '' };
  }
  if (mon === 'xe_gtidle') return { text: '—', title: 'No GPU samples captured' };
  if (mon === 'intel_xe_no_sysfs') return { text: '—', title: 'xe sysfs unavailable' };
  if (mon === 'intel_sysfs_unavailable') return { text: '—', title: 'Install intel-gpu-tools' };
  return { text: '—', title: '' };
}

function metricBar(value, max, kind, format) {
  const pct = max > 0 ? Math.min(100, (value / max) * 100) : 0;
  const label = format(value);
  return `<div class="metric-bar" title="${label}">
    <div class="metric-bar-track"><div class="metric-bar-fill ${kind}" style="width:${pct}%"></div></div>
    <span class="metric-bar-value">${label}</span>
  </div>`;
}

function rowLabel(row) {
  const accel = row.accel && row.accel !== 'software' ? row.accel : '';
  return accel ? `${row.fixture} · ${row.mode} · ${accel}` : `${row.fixture} · ${row.mode}`;
}

function renderReport(r) {
  const s = r.summary || {};
  document.getElementById('meta').textContent =
    `Generated ${r.generatedAt} · reference: ${r.referenceVideo} · ${r.fixtureDurationSec}s fixtures · ${r.segmentCount} segments/test`;

  document.getElementById('summary-cards').innerHTML = [
    card('Tests', s.totalTests),
    card('Passed', s.passed, 'pass'),
    card('Failed', s.failed, 'fail'),
    card('Skipped', s.skipped),
    card('Fixtures OK', s.fixturesGenerated),
    card('Fixtures failed', s.fixturesFailed, s.fixturesFailed ? 'fail' : ''),
  ].join('');

  document.getElementById('hardware').textContent = JSON.stringify(r.hardware, null, 2);

  const ftbody = document.querySelector('#fixture-table tbody');
  ftbody.innerHTML = (r.fixtures || []).map(f => {
    const st = f.error ? `<span class="fail">FAIL</span>` : (f.skipped ? 'cached' : '<span class="pass">OK</span>');
    return `<tr><td>${f.spec?.name || ''}</td><td class="mono">${f.path || ''}</td><td>${st}</td><td>${f.generateMs || ''}ms</td></tr>`;
  }).join('');

  const results = (r.results || []).filter(x => !x.skipped);
  const maxEncode = Math.max(1, ...results.map(x => x.totalEncodeMs || 0));
  const maxCpu = Math.max(1, ...results.map(x => x.resources?.cpuPercentAvg || 0));
  const gpuRows = results.filter(x => gpuPercent(x) != null);
  const maxGpu = Math.max(1, ...gpuRows.map(x => gpuPercent(x)));

  const tbody = document.querySelector('#results-table tbody');
  tbody.innerHTML = results.map(row => {
    const avg = avgSegMs(row);
    const cold = row.timing?.coldSegMs;
    const warm = row.timing?.warmAvgSegMs;
    const pass = row.pass ? '<span class="pass">PASS</span>' : '<span class="fail">FAIL</span>';
    const play = row.playbackUrl
      ? `<a href="player.html?playlist=${encodeURIComponent(row.playbackUrl)}&label=${encodeURIComponent(row.fixture + ' ' + row.label)}">Play</a>`
      : '';
    const gpu = gpuLabel(row);
    const hw = row.hw?.expectedAccel === 'software' || row.mode === 'remux' || row.mode === 'copy'
      ? 'sw'
      : (row.hw?.hwLikelyActive ? '<span class="pass">yes</span>' : '<span class="fail">no</span>');
    const gpuVal = gpuPercent(row);
    return `<tr>
      <td>${row.fixture}</td>
      <td>${row.mode}</td>
      <td>${row.accel}</td>
      <td>${pass}</td>
      <td class="mono">${row.hw?.encoder || ''}</td>
      <td>${hw}</td>
      <td class="metric-col">${metricBar(row.totalEncodeMs || 0, maxEncode, 'encode', v => `${Math.round(v)}ms`)}</td>
      <td>${avg}ms</td>
      <td>${cold != null ? `${cold}ms` : '—'}</td>
      <td>${warm != null ? `${warm}ms` : '—'}</td>
      <td class="metric-col">${metricBar(row.resources?.cpuPercentAvg || 0, maxCpu, 'cpu', v => `${v.toFixed(0)}%`)}</td>
      <td class="metric-col" title="${gpu.title}">${gpuVal != null
        ? metricBar(gpuVal, maxGpu, 'gpu', v => `${v.toFixed(0)}%`)
        : `<span class="muted">${gpu.text}</span>`}</td>
      <td>${(row.issues || []).length}</td>
      <td>${play}</td>
    </tr>`;
  }).join('');

  renderCharts(r, results, maxEncode, maxCpu, maxGpu);
}

function avgSegMs(row) {
  if (!row.segments?.length) return '—';
  const sum = row.segments.reduce((a, s) => a + (s.encodeMs || 0), 0);
  return Math.round(sum / row.segments.length);
}

function destroyCharts() {
  for (const c of chartInstances) c.destroy();
  chartInstances = [];
}

function setChartHeight(canvasId, rowCount, minHeight = 420) {
  const wrap = document.getElementById(canvasId)?.closest('.chart-wrap');
  if (!wrap) return;
  const h = Math.max(minHeight, rowCount * 26 + 80);
  wrap.style.height = `${h}px`;
}

function horizontalBarChart(canvasId, labels, values, label, color, maxValue) {
  setChartHeight(canvasId, labels.length);
  const ctx = document.getElementById(canvasId);
  const chart = new Chart(ctx, {
    type: 'bar',
    data: {
      labels,
      datasets: [{
        label,
        data: values,
        backgroundColor: color,
        borderRadius: 3,
      }],
    },
    options: {
      indexAxis: 'y',
      responsive: true,
      maintainAspectRatio: false,
      plugins: {
        legend: { display: false },
        tooltip: {
          callbacks: {
            label: (ctx) => `${label}: ${ctx.formattedValue}`,
          },
        },
      },
      scales: {
        x: {
          beginAtZero: true,
          suggestedMax: maxValue ? maxValue * 1.05 : undefined,
          grid: { color: '#2a2f3a' },
          ticks: { color: '#9aa0a6' },
        },
        y: {
          grid: { display: false },
          ticks: { color: '#e8eaed', font: { size: 11 }, autoSkip: false },
        },
      },
    },
  });
  chartInstances.push(chart);
}

function renderCharts(r, results, maxEncode, maxCpu, maxGpu) {
  destroyCharts();

  const passed = results.filter(x => x.pass).length;
  const failed = results.filter(x => !x.pass).length;
  const skipped = (r.results || []).filter(x => x.skipped).length;

  chartInstances.push(new Chart(document.getElementById('chart-pass'), {
    type: 'doughnut',
    data: {
      labels: ['Pass', 'Fail', 'Skip'],
      datasets: [{ data: [passed, failed, skipped], backgroundColor: ['#3d9970', '#ff4136', '#aaa'] }],
    },
    options: {
      maintainAspectRatio: false,
      plugins: { title: { display: true, text: 'Test outcomes', color: '#e8eaed' }, legend: { labels: { color: '#e8eaed' } } },
    },
  }));

  const byEncode = [...results].sort((a, b) => (b.totalEncodeMs || 0) - (a.totalEncodeMs || 0));
  horizontalBarChart(
    'chart-encode',
    byEncode.map(rowLabel),
    byEncode.map(x => x.totalEncodeMs || 0),
    'Total encode ms',
    '#0074d9',
    maxEncode,
  );

  const byCpu = [...results].sort((a, b) => (b.resources?.cpuPercentAvg || 0) - (a.resources?.cpuPercentAvg || 0));
  horizontalBarChart(
    'chart-cpu',
    byCpu.map(rowLabel),
    byCpu.map(x => x.resources?.cpuPercentAvg || 0),
    'CPU % avg',
    '#ff851b',
    maxCpu,
  );

  const byGpu = results
    .filter(x => gpuPercent(x) != null)
    .sort((a, b) => gpuPercent(b) - gpuPercent(a));
  horizontalBarChart(
    'chart-gpu',
    byGpu.map(rowLabel),
    byGpu.map(x => gpuPercent(x)),
    'GPU % avg',
    '#2ecc40',
    100,
  );
}

loadReport().catch(err => {
  document.getElementById('meta').textContent = 'Failed to load report: ' + err.message;
});
