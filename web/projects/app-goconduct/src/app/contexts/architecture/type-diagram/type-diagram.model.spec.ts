import { fakeIncomingTypeRelation, fakeTypeDeclaration } from '../../../testing/fake-clients';
import {
  buildTypeDiagramModel,
  TYPE_HEADER_HEIGHT,
  typeShortName,
} from './type-diagram.model';

const COMPONENT = 'internal/module/orders';

const TYPES = [
  fakeTypeDeclaration(`${COMPONENT}.Order`, 'struct', {
    component: COMPONENT,
    fields: [
      { name: 'Recorder', type: 'telemetry.Recorder', embedded: true, exported: true },
      { name: 'Sink', type: 'telemetry.Writer', embedded: false, exported: true },
      { name: 'Lines', type: '[]Line', embedded: false, exported: true },
    ],
    methods: [{ name: 'Total', signature: '() int', exported: true, pointerReceiver: false }],
    embeds: [
      { id: 'internal/library/telemetry.Recorder', component: 'internal/library/telemetry' },
    ],
    implements: [
      { id: 'internal/library/telemetry.Writer', component: 'internal/library/telemetry' },
    ],
    references: [{ id: `${COMPONENT}.Line`, component: COMPONENT }],
  }),
  fakeTypeDeclaration(`${COMPONENT}.Line`, 'struct', {
    component: COMPONENT,
    fields: [{ name: 'Quantity', type: 'int', embedded: false, exported: true }],
  }),
];

describe('buildTypeDiagramModel', () => {
  it('draws one node per declared type, in identifier order', () => {
    const model = buildTypeDiagramModel(TYPES, [], new Set(), '');

    expect(model.nodes.map((node) => node.id)).toEqual([
      `${COMPONENT}.Line`,
      `${COMPONENT}.Order`,
    ]);
    expect(model.nodes[1]?.kind).toBe('struct');
  });

  it('gives every field and method row a port of its own', () => {
    const model = buildTypeDiagramModel(TYPES, [], new Set(), '');

    const order = model.nodes.find((node) => node.id === `${COMPONENT}.Order`);
    expect(order?.rows.map((row) => row.port)).toEqual([
      'field:Recorder',
      'field:Sink',
      'field:Lines',
      'method:Total',
    ]);
    expect(order?.rows[3]?.text).toBe('+ Total() int');
  });

  /**
   * Go states visibility through the capital initial, which a reader should
   * not have to inspect. Every row therefore carries the UML marker: a plus
   * for an exported member, a minus for an unexported one.
   */
  it('marks the visibility of every member with the UML prefix', () => {
    const withPrivate = fakeTypeDeclaration(`${COMPONENT}.cache`, 'struct', {
      component: COMPONENT,
      fields: [{ name: 'entries', type: 'map[string]Line', embedded: false, exported: false }],
      methods: [{ name: 'Len', signature: '() int', exported: true, pointerReceiver: true }],
    });

    const model = buildTypeDiagramModel([withPrivate], [], new Set(), '');

    const rows = model.nodes[0]?.rows;
    expect(rows?.[0]?.text).toBe('− entries: map[string]Line');
    expect(rows?.[1]?.text).toBe('+ Len() int');
  });

  it('draws a cross-component target as an external node with its component identifier', () => {
    const model = buildTypeDiagramModel(TYPES, [], new Set(), '');

    expect(model.externals.map((node) => node.id)).toEqual([
      'internal/library/telemetry.Recorder',
      'internal/library/telemetry.Writer',
    ]);
    expect(model.externals[0]?.componentId).toBe('internal/library/telemetry');
  });

  it('draws one edge per relation, each with its kind', () => {
    const model = buildTypeDiagramModel(TYPES, [], new Set(), '');

    expect(model.edges.map((edge) => edge.kind).sort()).toEqual([
      'embeds',
      'implements',
      'references',
    ]);
    const embed = model.edges.find((edge) => edge.kind === 'embeds');
    expect(embed?.sourcePort).toBe('field:Recorder');
    const reference = model.edges.find((edge) => edge.kind === 'references');
    expect(reference?.sourcePort).toBe('field:Lines');
    const implementation = model.edges.find((edge) => edge.kind === 'implements');
    expect(implementation?.sourcePort).toBeUndefined();
  });

  it('reduces a collapsed type to its header and detaches its member ports', () => {
    const collapsed = buildTypeDiagramModel(TYPES, [], new Set([`${COMPONENT}.Order`]), '');

    const order = collapsed.nodes.find((node) => node.id === `${COMPONENT}.Order`);
    expect(order?.collapsed).toBe(true);
    expect(order?.rows).toHaveLength(0);
    expect(order?.height).toBe(TYPE_HEADER_HEIGHT);
    const embed = collapsed.edges.find((edge) => edge.kind === 'embeds');
    expect(embed?.sourcePort).toBeUndefined();
  });

  it('produces the same model for the same input', () => {
    const first = buildTypeDiagramModel(TYPES, [], new Set(), '');
    const second = buildTypeDiagramModel(TYPES, [], new Set(), '');

    expect(first).toEqual(second);
  });

  /**
   * Go satisfies interfaces implicitly, so the component that declares one
   * never lists its implementers. A popular interface would drown the canvas
   * in arrows, so an unselected target only counts its incoming uses; the
   * selection draws its sources as external nodes at the left, with the arrow
   * keeping the Go direction: the implementer aims at the interface.
   */
  it('counts incoming uses on the target and draws them only for the selected type', () => {
    const incoming = [
      fakeIncomingTypeRelation(
        'implements',
        'internal/module/billing.Ledger',
        `${COMPONENT}.Order`,
      ),
    ];

    const unselected = buildTypeDiagramModel(TYPES, incoming, new Set(), '');
    const order = unselected.nodes.find((node) => node.id === `${COMPONENT}.Order`);
    expect(order?.incomingCount).toBe(1);
    expect(
      unselected.externals.some((node) => node.id === 'internal/module/billing.Ledger'),
    ).toBe(false);
    expect(unselected.edges.some((edge) => edge.id.startsWith('incoming:'))).toBe(false);

    const selected = buildTypeDiagramModel(TYPES, incoming, new Set(), `${COMPONENT}.Order`);
    const source = selected.externals.find(
      (node) => node.id === 'internal/module/billing.Ledger',
    );
    expect(source?.componentId).toBe('internal/module/billing');
    const edge = selected.edges.find((candidate) => candidate.id.startsWith('incoming:'));
    expect(edge?.kind).toBe('implements');
    expect(edge?.sourceId).toBe('internal/module/billing.Ledger');
    expect(edge?.targetId).toBe(`${COMPONENT}.Order`);
    // The incoming column opens at the left, so the internal nodes shift right.
    const internal = selected.nodes[0];
    expect(source !== undefined && internal !== undefined && source.x < internal.x).toBe(true);
  });
});

describe('typeShortName', () => {
  it('keeps the part after the package path', () => {
    expect(typeShortName('internal/module/orders.Order')).toBe('Order');
    expect(typeShortName('Order')).toBe('Order');
  });
});
