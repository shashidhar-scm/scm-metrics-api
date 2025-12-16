import { Component, OnDestroy, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ApiService, LatestMetric, SeriesPoint } from './api.service';
import { Subscription, combineLatest, timer } from 'rxjs';
import { switchMap } from 'rxjs/operators';
import Chart from 'chart.js/auto';

function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0B';
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB'];
  let b = bytes;
  let u = 0;
  while (b >= 1024 && u < units.length - 1) {
    b /= 1024;
    u++;
  }
  return `${b.toFixed(u === 0 ? 0 : 2)}${units[u]}`;
}

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './app.component.html',
})
export class AppComponent implements OnInit, OnDestroy {
  servers: string[] = [];
  selectedServer = '';
  ranges = [
    { label: 'Last 1h', value: '1h' },
    { label: 'Last 2h', value: '2h' },
    { label: 'Last 6h', value: '6h' },
    { label: 'Last 1d', value: '1d' },
  ];
  selectedRange = '6h';
  latest?: LatestMetric;

  private sub = new Subscription();
  private chartRefreshSub?: Subscription;
  private loadChart?: Chart;
  private diskChart?: Chart;
  private diskUsedChart?: Chart;

  constructor(private api: ApiService) {}

  ngOnInit(): void {
    this.sub.add(
      this.api.getServers().subscribe((servers: string[]) => {
        this.servers = servers;
        if (!this.selectedServer && servers.length > 0) {
          this.selectedServer = servers[0];
          this.onServerChange(this.selectedServer);
        }
      })
    );

    // Refresh latest summary every 30s
    this.sub.add(
      timer(0, 30000)
        .pipe(switchMap(() => combineLatest([this.api.getLatest()])))
        .subscribe(([latestAll]: [LatestMetric[]]) => {
          if (!this.selectedServer) return;
          this.latest = latestAll.find((m: LatestMetric) => m.server_id === this.selectedServer);
        })
    );
  }

  onServerChange(value: string): void {
    this.selectedServer = value;

    // Refresh charts when server changes
    this.refreshCharts();

    this.sub.add(
      this.api.getLatest().subscribe((latestAll: LatestMetric[]) => {
        this.latest = latestAll.find((m: LatestMetric) => m.server_id === this.selectedServer);
      })
    );
  }

  onRangeChange(value: string): void {
    this.selectedRange = value;
    this.refreshCharts();
  }

  private refreshCharts(): void {
    if (!this.selectedServer) return;

    this.chartRefreshSub?.unsubscribe();

    const range = this.selectedRange;
    this.chartRefreshSub = timer(0, 30000)
      .pipe(
        switchMap(() =>
          combineLatest([
            this.api.querySeries(this.selectedServer, 'system', 'load1', range),
            this.api.querySeries(this.selectedServer, 'disk', 'used_percent', range, { aggregated: true }),
            this.api.querySeries(this.selectedServer, 'disk', 'used', range, { aggregated: true }),
          ])
        )
      )
      .subscribe(([loadPoints, diskPercentPoints, diskUsedPoints]: [SeriesPoint[], SeriesPoint[], SeriesPoint[]]) => {
        this.renderLoadChart(loadPoints);
        this.renderDiskChart(diskPercentPoints);
        this.renderDiskUsedChart(diskUsedPoints);
      });

    this.sub.add(this.chartRefreshSub);
  }

  private renderLoadChart(points: SeriesPoint[]): void {
    const labels = points.map((p) => new Date(p.time).toLocaleTimeString());
    const data = points.map((p) => p.value_double ?? 0);

    const ctx = (document.getElementById('loadChart') as HTMLCanvasElement | null)?.getContext('2d');
    if (!ctx) return;

    this.loadChart?.destroy();
    this.loadChart = new Chart(ctx, {
      type: 'line',
      data: {
        labels,
        datasets: [
          {
            label: 'Load 1m',
            data,
            borderColor: '#7aa2ff',
            backgroundColor: 'rgba(122,162,255,0.15)',
            fill: true,
            tension: 0.25,
            pointRadius: 0,
          },
        ],
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
          legend: { display: true },
        },
        scales: {
          x: { ticks: { color: '#a9b4d0' }, grid: { color: 'rgba(255,255,255,0.06)' } },
          y: { ticks: { color: '#a9b4d0' }, grid: { color: 'rgba(255,255,255,0.06)' } },
        },
      },
    });
  }

  private renderDiskUsedChart(points: SeriesPoint[]): void {
    const labels = points.map((p) => new Date(p.time).toLocaleTimeString());
    const data = points.map((p) => Number(p.value_int ?? 0));

    const ctx = (document.getElementById('diskUsedChart') as HTMLCanvasElement | null)?.getContext('2d');
    if (!ctx) return;

    this.diskUsedChart?.destroy();
    this.diskUsedChart = new Chart(ctx, {
      type: 'line',
      data: {
        labels,
        datasets: [
          {
            label: 'Disk Used (Bytes, Aggregated)',
            data,
            borderColor: '#fbbf24',
            backgroundColor: 'rgba(251,191,36,0.12)',
            fill: true,
            tension: 0.25,
            pointRadius: 0,
          },
        ],
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
          legend: { display: true },
        },
        scales: {
          x: { ticks: { color: '#a9b4d0' }, grid: { color: 'rgba(255,255,255,0.06)' } },
          y: {
            ticks: {
              color: '#a9b4d0',
              callback: (v) => formatBytes(Number(v)),
            },
            grid: { color: 'rgba(255,255,255,0.06)' },
          },
        },
      },
    });
  }

  private renderDiskChart(points: SeriesPoint[]): void {
    const labels = points.map((p) => new Date(p.time).toLocaleTimeString());
    const data = points.map((p) => p.value_double ?? 0);

    const ctx = (document.getElementById('diskChart') as HTMLCanvasElement | null)?.getContext('2d');
    if (!ctx) return;

    this.diskChart?.destroy();
    this.diskChart = new Chart(ctx, {
      type: 'line',
      data: {
        labels,
        datasets: [
          {
            label: 'Disk Used % (Aggregated)',
            data,
            borderColor: '#34d399',
            backgroundColor: 'rgba(52,211,153,0.15)',
            fill: true,
            tension: 0.25,
            pointRadius: 0,
          },
        ],
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
          legend: { display: true },
        },
        scales: {
          x: { ticks: { color: '#a9b4d0' }, grid: { color: 'rgba(255,255,255,0.06)' } },
          y: {
            ticks: { color: '#a9b4d0' },
            grid: { color: 'rgba(255,255,255,0.06)' },
            suggestedMin: 0,
            suggestedMax: 100,
          },
        },
      },
    });
  }

  memoryUsedTotal(): string {
    if (!this.latest) return '-';
    return `${formatBytes(this.latest.memory_used_bytes)}/${formatBytes(this.latest.memory_total_bytes)}`;
  }

  diskUsedTotal(): string {
    if (!this.latest) return '-';
    return `${formatBytes(this.latest.disk_used_bytes)}/${formatBytes(this.latest.disk_total_bytes)}`;
  }

  ngOnDestroy(): void {
    this.sub.unsubscribe();
    this.loadChart?.destroy();
    this.diskChart?.destroy();
    this.diskUsedChart?.destroy();
  }
}
