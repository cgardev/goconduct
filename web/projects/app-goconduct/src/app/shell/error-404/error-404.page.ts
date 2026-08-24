import { Component } from '@angular/core';
import { RouterLink } from '@angular/router';
import { TuiButton } from '@taiga-ui/core';
import { navTo } from '../../kernel/routing/app-navigation';

/** In-shell page for a URL that matches no route. */
@Component({
  selector: 'app-error-404-page',
  imports: [RouterLink, TuiButton],
  templateUrl: './error-404.page.html',
  styleUrl: './error-404.page.less',
})
export class Error404Page {
  protected readonly overviewLink = navTo.overview();
}
