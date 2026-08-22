import { provideZonelessChangeDetection } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter, withComponentInputBinding } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import { fakeComponent, fakeGraph, provideFakeClients } from '../../../../testing/fake-clients';
import { ComponentsPage } from './components.page';

const COMPONENTS = [
  fakeComponent('internal/library/clock', 'library', 2),
  fakeComponent('internal/library/eventbus', 'library', 7),
  fakeComponent('cmd/goconduct', 'application', 1),
];

interface Rendered {
  readonly element: HTMLElement;
  readonly harness: RouterTestingHarness;
}

async function renderPage(url = '/components'): Promise<Rendered> {
  TestBed.configureTestingModule({
    providers: [
      provideZonelessChangeDetection(),
      provideRouter(
        [{ path: 'components', component: ComponentsPage }],
        withComponentInputBinding(),
      ),
      ...(provideFakeClients(fakeGraph({ components: COMPONENTS })) as never[]),
    ],
  });

  const harness = await RouterTestingHarness.create();
  await harness.navigateByUrl(url, ComponentsPage);
  // The store resolves its first graph in a promise, so the rendered table only
  // holds rows after the microtask queue drains.
  await new Promise((resolve) => setTimeout(resolve, 0));
  harness.detectChanges();
  return { element: harness.routeNativeElement as HTMLElement, harness };
}

/** Types into a control the way a reader does, one whole value at a time. */
function type(element: HTMLElement, selector: string, value: string): void {
  const input = element.querySelector<HTMLInputElement>(selector);
  if (input === null) {
    throw new Error(`no control matches ${selector}`);
  }
  input.value = value;
  input.dispatchEvent(new Event('input'));
}

describe('ComponentsPage', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('renders one row per analyzed component', async () => {
    const { element } = await renderPage();

    expect(element.querySelectorAll('tbody tr')).toHaveLength(COMPONENTS.length);
  });

  /**
   * The role filter used to be a native select bound through its `value`
   * attribute, which the browser ignores once the options render after it. The
   * control then named a role the table was not filtered by.
   */
  it('shows the role from the address in the filter control', async () => {
    const { element } = await renderPage('/components?role=library');
    const trigger = element.querySelector<HTMLInputElement>('.components__role input');

    expect(trigger?.value).toBe('library');
    expect(element.querySelectorAll('tbody tr')).toHaveLength(2);
  });

  it('shows the query from the address in the search control', async () => {
    const { element } = await renderPage('/components?q=eventbus');
    const search = element.querySelector<HTMLInputElement>('.components__search input');

    expect(search?.value).toBe('eventbus');
    expect(element.querySelectorAll('tbody tr')).toHaveLength(1);
  });

  it('offers a way out when the filters match nothing', async () => {
    const { element } = await renderPage('/components?q=absent');

    expect(element.querySelectorAll('tbody tr')).toHaveLength(0);
    expect(element.textContent).toContain('No matches');
    expect(element.textContent).toContain('Clear filters');
  });

  it('marks the sorted column for assistive technology', async () => {
    const { element } = await renderPage();
    const sorted = element.querySelectorAll('th[aria-sort]:not([aria-sort="none"])');

    expect(sorted).toHaveLength(1);
    expect(sorted[0]?.textContent).toContain('Ca');
  });

  it('counts the components beside the heading', async () => {
    const { element } = await renderPage();

    expect(element.querySelector('.page-header__counter')?.textContent).toContain('3');
  });

  /**
   * The filters used to be read straight from the address, and a navigation
   * settles in a promise. Every keystroke therefore raced the router, and the
   * search field could show a value the table did not answer to.
   */
  it('narrows the table as the reader types, without waiting for the address', async () => {
    const { element, harness } = await renderPage();

    type(element, '.components__search input', 'clock');
    harness.detectChanges();

    expect(element.querySelectorAll('tbody tr')).toHaveLength(1);
    expect(element.querySelector<HTMLInputElement>('.components__search input')?.value).toBe(
      'clock',
    );
  });

  it('restores every component when the filters are cleared', async () => {
    const { element, harness } = await renderPage('/components?q=clock');
    expect(element.querySelectorAll('tbody tr')).toHaveLength(1);

    element.querySelector<HTMLButtonElement>('.components__clear')?.click();
    harness.detectChanges();

    expect(element.querySelectorAll('tbody tr')).toHaveLength(3);
  });
});
