import { Injectable } from '@angular/core';
import { HttpClient, HttpParams } from '@angular/common/http';
import { Observable } from 'rxjs';

export interface LatestMetric {
  server_id: string;
  time: string;
  cpu: number;
  memory: number;
  memory_total_bytes: number;
  memory_used_bytes: number;
  disk: number;
  disk_total_bytes: number;
  disk_used_bytes: number;
  disk_free_bytes: number;
}

export interface SeriesPoint {
  time: string;
  server_id: string;
  measurement: string;
  field: string;
  value_double?: number;
  value_int?: number;
  tags: Record<string, any>;
}

@Injectable({ providedIn: 'root' })
export class ApiService {
  constructor(private http: HttpClient) {}

  getServers(): Observable<string[]> {
    return this.http.get<string[]>('/api/servers');
  }

  getLatest(): Observable<LatestMetric[]> {
    return this.http.get<LatestMetric[]>('/api/metrics/latest');
  }

  querySeries(
    serverId: string,
    measurement: string,
    field: string,
    range: string,
    tags?: Record<string, any>
  ): Observable<SeriesPoint[]> {
    let params = new HttpParams()
      .set('server_id', serverId)
      .set('measurement', measurement)
      .set('field', field)
      .set('range', range);

    if (tags && Object.keys(tags).length > 0) {
      params = params.set('tags', JSON.stringify(tags));
    }

    return this.http.get<SeriesPoint[]>('/api/series/query', { params });
  }
}
