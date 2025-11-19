package api

import (
	"net/http"
)

func (s *Server) handleFrontend(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(frontendHTML))
}

const frontendHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>DevLog Dashboard</title>
    <script src="https://cdn.jsdelivr.net/npm/chart.js@4.4.1/dist/chart.umd.min.js"></script>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
            background: #0f0f0f;
            color: #e0e0e0;
            line-height: 1.6;
        }

        .container {
            max-width: 1400px;
            margin: 0 auto;
            padding: 20px;
        }

        header {
            background: #1a1a1a;
            padding: 20px;
            border-bottom: 2px solid #2a2a2a;
            margin-bottom: 30px;
        }

        h1 {
            font-size: 2em;
            font-weight: 600;
            color: #ffffff;
        }

        .subtitle {
            color: #888;
            font-size: 0.9em;
            margin-top: 5px;
        }

        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
            gap: 20px;
            margin-bottom: 30px;
        }

        .stat-card {
            background: #1a1a1a;
            padding: 20px;
            border-radius: 8px;
            border: 1px solid #2a2a2a;
        }

        .stat-card h3 {
            font-size: 0.9em;
            color: #888;
            text-transform: uppercase;
            letter-spacing: 0.5px;
            margin-bottom: 10px;
        }

        .stat-value {
            font-size: 2em;
            font-weight: 700;
            color: #2563eb;
        }

        .chart-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(500px, 1fr));
            gap: 20px;
            margin-bottom: 30px;
        }

        .chart-card {
            background: #1a1a1a;
            padding: 20px;
            border-radius: 8px;
            border: 1px solid #2a2a2a;
        }

        .chart-card h2 {
            font-size: 1.2em;
            margin-bottom: 15px;
            color: #ffffff;
        }

        .chart-container {
            position: relative;
            height: 300px;
        }

        .events-section {
            background: #1a1a1a;
            padding: 20px;
            border-radius: 8px;
            border: 1px solid #2a2a2a;
            margin-bottom: 30px;
        }

        .events-section h2 {
            font-size: 1.2em;
            margin-bottom: 15px;
            color: #ffffff;
        }

        .events-list {
            max-height: 400px;
            overflow-y: auto;
        }

        .event-item {
            padding: 10px;
            border-bottom: 1px solid #2a2a2a;
            font-size: 0.9em;
        }

        .event-item:last-child {
            border-bottom: none;
        }

        .event-time {
            color: #666;
            font-size: 0.85em;
        }

        .event-source {
            display: inline-block;
            padding: 2px 8px;
            border-radius: 4px;
            font-size: 0.8em;
            font-weight: 600;
            margin-right: 8px;
        }

        .source-git { background: #10b981; color: white; }
        .source-shell { background: #f59e0b; color: white; }
        .source-clipboard { background: #8b5cf6; color: white; }
        .source-tmux { background: #ec4899; color: white; }
        .source-wisprflow { background: #06b6d4; color: white; }

        .event-type {
            color: #888;
        }

        .event-details {
            color: #ccc;
            margin-top: 4px;
        }

        .loading {
            text-align: center;
            padding: 40px;
            color: #666;
        }

        .error {
            background: #dc2626;
            color: white;
            padding: 15px;
            border-radius: 8px;
            margin-bottom: 20px;
        }

        ::-webkit-scrollbar {
            width: 8px;
        }

        ::-webkit-scrollbar-track {
            background: #1a1a1a;
        }

        ::-webkit-scrollbar-thumb {
            background: #2a2a2a;
            border-radius: 4px;
        }

        ::-webkit-scrollbar-thumb:hover {
            background: #3a3a3a;
        }
    </style>
</head>
<body>
    <header>
        <div class="container">
            <h1>DevLog Dashboard</h1>
            <div class="subtitle">Local development activity tracking</div>
        </div>
    </header>

    <div class="container">
        <div id="error-container"></div>

        <div class="stats-grid">
            <div class="stat-card">
                <h3>Total Events</h3>
                <div class="stat-value" id="total-events">-</div>
            </div>
            <div class="stat-card">
                <h3>Uptime</h3>
                <div class="stat-value" id="uptime">-</div>
            </div>
        </div>

        <div class="chart-grid">
            <div class="chart-card">
                <h2>Events by Source</h2>
                <div class="chart-container">
                    <canvas id="source-chart"></canvas>
                </div>
            </div>
            <div class="chart-card">
                <h2>Activity Timeline (Last 7 Days)</h2>
                <div class="chart-container">
                    <canvas id="timeline-chart"></canvas>
                </div>
            </div>
        </div>

        <div class="chart-grid">
            <div class="chart-card">
                <h2>Top Repositories</h2>
                <div class="chart-container">
                    <canvas id="repo-chart"></canvas>
                </div>
            </div>
            <div class="chart-card">
                <h2>Most Used Commands</h2>
                <div class="chart-container">
                    <canvas id="command-chart"></canvas>
                </div>
            </div>
        </div>

        <div class="events-section">
            <h2>Recent Events (Last 50)</h2>
            <div id="events-list" class="events-list"></div>
        </div>
    </div>

    <script>
        let charts = {};

        function showError(message) {
            const container = document.getElementById('error-container');
            container.innerHTML = '<div class="error">' + message + '</div>';
        }

        function clearError() {
            document.getElementById('error-container').innerHTML = '';
        }

        async function fetchJSON(url) {
            const response = await fetch(url);
            if (!response.ok) {
                throw new Error('Failed to fetch ' + url);
            }
            return response.json();
        }

        async function loadStatus() {
            try {
                const data = await fetchJSON('/api/v1/status');
                document.getElementById('total-events').textContent = data.event_count.toLocaleString();

                const hours = Math.floor(data.uptime_seconds / 3600);
                const minutes = Math.floor((data.uptime_seconds % 3600) / 60);
                document.getElementById('uptime').textContent = hours + 'h ' + minutes + 'm';
            } catch (error) {
                console.error('Failed to load status:', error);
            }
        }

        async function loadEvents() {
            try {
                const data = await fetchJSON('/api/v1/events');
                const listEl = document.getElementById('events-list');

                if (data.events.length === 0) {
                    listEl.innerHTML = '<div class="event-item">No events found</div>';
                    return;
                }

                listEl.innerHTML = data.events.map(event => {
                    const time = new Date(event.timestamp).toLocaleString();
                    const sourceClass = 'source-' + event.source;

                    let details = '';
                    if (event.payload) {
                        if (event.payload.message) {
                            details = event.payload.message;
                        } else if (event.payload.command) {
                            details = event.payload.command;
                        } else if (event.payload.content) {
                            const content = event.payload.content;
                            details = content.length > 50 ? content.substring(0, 50) + '...' : content;
                        }
                    }

                    if (event.repo) {
                        details = (details ? details + ' • ' : '') + event.repo.split('/').pop();
                    }

                    return '<div class="event-item">' +
                        '<div>' +
                        '<span class="event-source ' + sourceClass + '">' + event.source + '</span>' +
                        '<span class="event-type">' + event.type + '</span>' +
                        '</div>' +
                        (details ? '<div class="event-details">' + details + '</div>' : '') +
                        '<div class="event-time">' + time + '</div>' +
                        '</div>';
                }).join('');
            } catch (error) {
                console.error('Failed to load events:', error);
                showError('Failed to load events: ' + error.message);
            }
        }

        async function loadEventsBySource() {
            try {
                const data = await fetchJSON('/api/v1/analytics/events-by-source');

                if (charts.sourceChart) {
                    charts.sourceChart.destroy();
                }

                const ctx = document.getElementById('source-chart').getContext('2d');
                charts.sourceChart = new Chart(ctx, {
                    type: 'doughnut',
                    data: {
                        labels: data.data.map(d => d.source),
                        datasets: [{
                            data: data.data.map(d => d.count),
                            backgroundColor: [
                                '#10b981',
                                '#f59e0b',
                                '#8b5cf6',
                                '#ec4899',
                                '#06b6d4',
                                '#6366f1'
                            ]
                        }]
                    },
                    options: {
                        responsive: true,
                        maintainAspectRatio: false,
                        plugins: {
                            legend: {
                                position: 'bottom',
                                labels: { color: '#e0e0e0' }
                            }
                        }
                    }
                });
            } catch (error) {
                console.error('Failed to load source data:', error);
            }
        }

        async function loadTimeline() {
            try {
                const data = await fetchJSON('/api/v1/analytics/events-timeline');

                if (charts.timelineChart) {
                    charts.timelineChart.destroy();
                }

                const reversedData = data.data.slice().reverse();

                const ctx = document.getElementById('timeline-chart').getContext('2d');
                charts.timelineChart = new Chart(ctx, {
                    type: 'line',
                    data: {
                        labels: reversedData.map(d => {
                            const date = new Date(d.hour);
                            return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric', hour: 'numeric' });
                        }),
                        datasets: [{
                            label: 'Events',
                            data: reversedData.map(d => d.count),
                            borderColor: '#2563eb',
                            backgroundColor: 'rgba(37, 99, 235, 0.1)',
                            fill: true,
                            tension: 0.4
                        }]
                    },
                    options: {
                        responsive: true,
                        maintainAspectRatio: false,
                        plugins: {
                            legend: {
                                display: false
                            }
                        },
                        scales: {
                            x: {
                                ticks: {
                                    color: '#888',
                                    maxRotation: 45,
                                    minRotation: 45
                                },
                                grid: { color: '#2a2a2a' }
                            },
                            y: {
                                ticks: { color: '#888' },
                                grid: { color: '#2a2a2a' }
                            }
                        }
                    }
                });
            } catch (error) {
                console.error('Failed to load timeline:', error);
            }
        }

        async function loadRepoStats() {
            try {
                const data = await fetchJSON('/api/v1/analytics/repo-stats');

                if (charts.repoChart) {
                    charts.repoChart.destroy();
                }

                if (data.data.length === 0) {
                    return;
                }

                const ctx = document.getElementById('repo-chart').getContext('2d');
                charts.repoChart = new Chart(ctx, {
                    type: 'bar',
                    data: {
                        labels: data.data.map(d => d.repo.split('/').pop()),
                        datasets: [{
                            label: 'Events',
                            data: data.data.map(d => d.count),
                            backgroundColor: '#10b981'
                        }]
                    },
                    options: {
                        responsive: true,
                        maintainAspectRatio: false,
                        indexAxis: 'y',
                        plugins: {
                            legend: { display: false }
                        },
                        scales: {
                            x: {
                                ticks: { color: '#888' },
                                grid: { color: '#2a2a2a' }
                            },
                            y: {
                                ticks: { color: '#888' },
                                grid: { display: false }
                            }
                        }
                    }
                });
            } catch (error) {
                console.error('Failed to load repo stats:', error);
            }
        }

        async function loadCommandStats() {
            try {
                const data = await fetchJSON('/api/v1/analytics/command-stats');

                if (charts.commandChart) {
                    charts.commandChart.destroy();
                }

                if (data.data.length === 0) {
                    return;
                }

                const ctx = document.getElementById('command-chart').getContext('2d');
                charts.commandChart = new Chart(ctx, {
                    type: 'bar',
                    data: {
                        labels: data.data.map(d => {
                            const cmd = d.command;
                            return cmd.length > 30 ? cmd.substring(0, 30) + '...' : cmd;
                        }),
                        datasets: [{
                            label: 'Count',
                            data: data.data.map(d => d.count),
                            backgroundColor: '#f59e0b'
                        }]
                    },
                    options: {
                        responsive: true,
                        maintainAspectRatio: false,
                        indexAxis: 'y',
                        plugins: {
                            legend: { display: false }
                        },
                        scales: {
                            x: {
                                ticks: { color: '#888' },
                                grid: { color: '#2a2a2a' }
                            },
                            y: {
                                ticks: { color: '#888' },
                                grid: { display: false }
                            }
                        }
                    }
                });
            } catch (error) {
                console.error('Failed to load command stats:', error);
            }
        }

        async function loadAllData() {
            clearError();
            try {
                await Promise.all([
                    loadStatus(),
                    loadEvents(),
                    loadEventsBySource(),
                    loadTimeline(),
                    loadRepoStats(),
                    loadCommandStats()
                ]);
            } catch (error) {
                showError('Failed to load dashboard data: ' + error.message);
            }
        }

        loadAllData();
        setInterval(loadAllData, 30000);
    </script>
</body>
</html>
`
