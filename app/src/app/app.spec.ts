import { TestBed } from '@angular/core/testing';
import { vi } from 'vitest';
import { App } from './app';
import { WailsIpcService } from './wails-ipc.service';

describe('App', () => {
  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [App],
    }).compileComponents();
  });

  it('should create the app', () => {
    const fixture = TestBed.createComponent(App);
    const app = fixture.componentInstance;
    expect(app).toBeTruthy();
  });

  it('should render title', async () => {
    const fixture = TestBed.createComponent(App);
    await fixture.whenStable();
    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.querySelector('h1')?.textContent).toContain('TRAWL');
  });

  it('loads settings and hydrates every view on startup via refreshAll', () => {
    const fixture = TestBed.createComponent(App);
    const ipc = TestBed.inject(WailsIpcService);
    const loadSettings = vi.spyOn(ipc, 'loadSettings').mockResolvedValue();
    const refreshAll = vi.spyOn(ipc, 'refreshAll').mockResolvedValue();

    fixture.componentInstance.ngOnInit();

    expect(loadSettings).toHaveBeenCalledTimes(1);
    expect(refreshAll).toHaveBeenCalledTimes(1);
  });
});
