import {
  provideBrowserGlobalErrorListeners,
  provideZonelessChangeDetection,
} from '@angular/core';
import { bootstrapApplication } from '@angular/platform-browser';
import { provideTaiga } from '@taiga-ui/core';
import { App } from './app/app';
import { provideGraphClient } from './app/api/graph-client';
import { provideQualityClient } from './app/api/quality-client';

void bootstrapApplication(App, {
  providers: [
    provideBrowserGlobalErrorListeners(),
    provideZonelessChangeDetection(),
    provideTaiga(),
    provideGraphClient(),
    provideQualityClient(),
  ],
}).catch((error: unknown) => console.error(error));
