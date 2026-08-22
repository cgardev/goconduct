import { ChangeDetectionStrategy, Component, computed, inject, input } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ActivatedRoute, Router } from '@angular/router';
import { TuiButton, TuiDropdown, TuiIcon, TuiLoader, TuiTextfield } from '@taiga-ui/core';
import { TuiChevron, TuiDataListWrapper, TuiSelect } from '@taiga-ui/kit';
import { GraphStore } from '../../../../kernel/graph/graph.store';
import { AlertComponent } from '../../../../ui/alert/alert.component';
import { EmptyStateComponent } from '../../../../ui/empty-state/empty-state.component';
import { PageHeaderComponent } from '../../../../ui/page-header/page-header.component';

/** Value of the severity filter that keeps every finding. */
const EVERY_SEVERITY = 'all';

/** Severities the filter offers, with the catch-all value first. */
const SEVERITY_OPTIONS = [EVERY_SEVERITY, 'error', 'warning'] as const;

/** Labels of the severity filter. */
const SEVERITY_LABELS: Record<string, string> = {
  all: 'All severities',
  error: 'Errors',
  warning: 'Warnings',
};

/**
 * Results of the deterministic architecture rules.
 *
 * The severity travels in the query string for the same reason the components
 * filters do: a reader who found an error worth reporting can paste the address
 * and the reader who opens it sees the same list.
 */
@Component({
  selector: 'app-findings-page',
  imports: [
    AlertComponent,
    EmptyStateComponent,
    FormsModule,
    PageHeaderComponent,
    TuiButton,
    TuiChevron,
    TuiDataListWrapper,
    TuiDropdown,
    TuiIcon,
    TuiLoader,
    TuiSelect,
    TuiTextfield,
  ],
  templateUrl: './findings.page.html',
  styleUrl: './findings.page.less',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class FindingsPage {
  private readonly router = inject(Router);
  private readonly route = inject(ActivatedRoute);

  protected readonly graph = inject(GraphStore);

  /**
   * Selected severity, bound from the `severity` query parameter.
   *
   * The router writes `undefined` when the parameter is absent, which overrides
   * the declared default, so the value is normalized below rather than read
   * directly.
   */
  readonly severity = input<string | undefined>(EVERY_SEVERITY);

  protected readonly severityOptions = SEVERITY_OPTIONS;

  // An absent parameter and an empty one mean the same thing to a reader, so
  // both resolve to the value that keeps every severity.
  protected readonly selectedSeverity = computed(() => this.severity() || EVERY_SEVERITY);

  /** Whether the reader has narrowed the list. */
  protected readonly filtered = computed(() => this.selectedSeverity() !== EVERY_SEVERITY);

  /** The findings that match the selected severity. */
  protected readonly matches = computed(() => {
    const severity = this.selectedSeverity();
    if (severity === EVERY_SEVERITY) {
      return this.graph.findings();
    }
    return this.graph.findings().filter((finding) => finding.severity === severity);
  });

  /** How many findings match, stated beside the heading. */
  protected readonly counter = computed(() => {
    const total = this.graph.findings().length;
    return this.filtered() ? `(${this.matches().length} of ${total})` : `(${total})`;
  });

  /** Names the selected severity in the trigger and in the dropdown. */
  protected readonly severityLabel = (value: string): string => SEVERITY_LABELS[value] ?? value;

  /** Icon that states the severity of one finding. */
  protected severityIcon(severity: string): string {
    return severity === 'error' ? '@tui.circle-x' : '@tui.triangle-alert';
  }

  protected setSeverity(value: string | null): void {
    void this.router.navigate([], {
      relativeTo: this.route,
      queryParams: { severity: value === null || value === EVERY_SEVERITY ? null : value },
      queryParamsHandling: 'merge',
      replaceUrl: true,
    });
  }

  protected clearFilters(): void {
    this.setSeverity(null);
  }
}
