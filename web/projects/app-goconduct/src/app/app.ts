import { Component } from '@angular/core';
import { RouterOutlet } from '@angular/router';
import { TuiRoot } from '@taiga-ui/core';

/**
 * Application root. Everything renders inside `<tui-root>`, which is both the
 * layout frame and the portal host Taiga projects its dialogs, dropdowns, and
 * notifications into. Without it those surfaces have nowhere to open.
 */
@Component({
  selector: 'app-root',
  imports: [RouterOutlet, TuiRoot],
  template: '<tui-root><router-outlet /></tui-root>',
  styles: `
    :host {
      display: flex;
      width: 100%;
      min-height: 100%;
    }
  `,
})
export class App {}
