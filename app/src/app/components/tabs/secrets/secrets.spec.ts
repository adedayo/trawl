import { ComponentFixture, TestBed } from '@angular/core/testing';

import { SecretsComponent } from './secrets';

describe('Secrets', () => {
  let component: SecretsComponent;
  let fixture: ComponentFixture<SecretsComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [SecretsComponent],
    }).compileComponents();

    fixture = TestBed.createComponent(SecretsComponent);
    component = fixture.componentInstance;
    await fixture.whenStable();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
