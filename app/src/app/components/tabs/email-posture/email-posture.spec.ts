import { ComponentFixture, TestBed } from '@angular/core/testing';

import { EmailPostureComponent } from './email-posture';

describe('EmailPosture', () => {
  let component: EmailPostureComponent;
  let fixture: ComponentFixture<EmailPostureComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [EmailPostureComponent],
    }).compileComponents();

    fixture = TestBed.createComponent(EmailPostureComponent);
    component = fixture.componentInstance;
    await fixture.whenStable();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
