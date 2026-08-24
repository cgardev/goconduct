# Preferencias de bibliotecas

Este documento fija las bibliotecas y las reglas visuales del frontend.

## Asignación obligatoria

| Visualización concreta | Biblioteca | Implementación |
| --- | --- | --- |
| Secuencia principal | amCharts 5 | Usa un gráfico XY con una serie de dispersión. |
| Evolución temporal de calidad, cobertura o acoplamiento | amCharts 5 | Usa un gráfico XY con eje de fecha y líneas. |
| Comparación de métricas entre componentes | amCharts 5 | Usa barras o columnas en un gráfico XY. |
| Distribución de complejidad, tamaño o cobertura | amCharts 5 | Usa un histograma en un gráfico XY. |
| Correlación entre dos métricas | amCharts 5 | Usa una serie de dispersión o burbujas. |
| Proporción de hallazgos, tipos o dependencias | amCharts 5 | Usa sectores o anillos cuando la suma represente un total. |
| Intensidad numérica entre dos dimensiones | amCharts 5 | Usa un mapa de calor. |
| Tamaño relativo de paquetes o componentes | amCharts 5 | Usa un mapa de árbol cuando el área represente una métrica. |
| Flujo agregado entre componentes | amCharts 5 | Usa Sankey solo cuando el ancho represente una cantidad. |
| Mapa de dependencias del repositorio | `@joint/core` | Usa nodos dirigidos para componentes, paquetes o módulos. |
| Diagrama UML de tipos Go | `@joint/core` | Usa nodos personalizados con campos, métodos e interfaces. |
| Implementación y composición de interfaces | `@joint/core` | Usa enlaces semánticos con estilos diferentes. |
| Grafo de llamadas entre funciones o métodos | `@joint/core` | Usa enlaces interactivos entre miembros concretos. |
| Lugares de uso de una función o un método | `@joint/core` | Resalta enlaces y abre el detalle del código. |
| Tarjetas con una cifra y un texto | Angular y Taiga UI | No uses una biblioteca de gráficos. |
| Tablas, filtros, búsquedas y paneles | Taiga UI | No uses amCharts ni JointJS. |
| Indicadores simples de estado o progreso | Taiga UI | Usa un componente de interfaz sin crear un gráfico. |

## Regla de decisión

- Usa amCharts 5 cuando la posición, el tamaño, el color o el ancho representen valores numéricos.
- Usa JointJS cuando los nodos representen código y sus enlaces tengan significado estructural.
- Usa Taiga UI cuando la información no necesite ejes, coordenadas o una topología de nodos.
- Usa JointJS para un grafo navegable, aunque los enlaces también tengan recuentos.
- Usa amCharts para un flujo agregado cuando los nodos no muestren campos ni métodos.

## amCharts 5: gráficos cuantitativos

- Usa amCharts 5 para métricas, series temporales, distribuciones y otros gráficos estadísticos.
- No uses amCharts 4 ni mezcles ambas versiones.

### Uso obligatorio con IA

- Usa siempre el [MCP oficial de amCharts](https://www.amcharts.com/docs/v5/ai/mcp/) antes de crear o modificar un gráfico.
- Conecta el cliente de IA con `https://mcp.amcharts.com/mcp`.
- Prefiere el servidor alojado porque no necesita instalación y mantiene la documentación actualizada.
- Usa `npx -y @amcharts/amcharts5-mcp` cuando el cliente solo permita un servidor local.
- No generes código de amCharts si el MCP no está disponible.
- Restablece primero la conexión con el MCP.
- No sustituyas el MCP con el conocimiento previo del modelo.
- Usa la búsqueda web y la habilidad oficial solo como apoyos adicionales.

### Proceso obligatorio para la IA

1. Confirma la conexión con el MCP antes de escribir código.
2. Consulta la referencia principal de amCharts 5.
3. Identifica el tipo exacto de gráfico solicitado.
4. Consulta la referencia específica para ese tipo mediante el MCP.
5. Busca mediante el MCP los ejemplos, las clases y las opciones necesarias.
6. Solicita una plantilla mínima cuando no exista un ejemplo local adecuado.
7. Comprueba cada API dudosa contra la documentación obtenida mediante el MCP.
8. Implementa el gráfico con Angular y TypeScript.
9. Devuelve cualquier error exacto al MCP y consulta la corrección documentada.
10. Repite la comprobación después de cambiar el tipo o una función importante.

La IA selecciona las herramientas del MCP y carga solo la información pertinente.

### Herramientas del MCP

Usa cada herramienta para su finalidad documentada:

| Herramienta | Finalidad |
| --- | --- |
| `get_core_reference` | Consultar la configuración, los temas, los colores, los eventos y los errores frecuentes. |
| `get_chart_reference` | Obtener la referencia completa de un tipo de gráfico. |
| `list_chart_types` | Identificar los tipos disponibles y sus palabras clave. |
| `search_docs` | Buscar una palabra dentro de las referencias estructuradas. |
| `search_all` | Buscar en la documentación, los ejemplos y la referencia de API. |
| `get_doc` | Obtener una página completa de documentación. |
| `get_section` | Obtener una sección concreta por su encabezado. |
| `get_quick_start` | Obtener una plantilla funcional mínima para un tipo de gráfico. |
| `get_api_reference` | Comprobar las opciones, los valores iniciales y la API de una clase. |
| `list_examples` | Localizar ejemplos por categoría. |
| `get_example` | Obtener el código completo de un ejemplo. |

### Contexto obligatorio de cada solicitud

Cada solicitud para una IA debe indicar estos datos:

- amCharts 5 como única versión permitida.
- El tipo exacto de gráfico.
- Angular como marco de trabajo.
- TypeScript como lenguaje.
- El componente completo o solo el código del gráfico.
- Las funciones necesarias, como leyendas, ayudas, barras de desplazamiento y animaciones.
- El MCP oficial como fuente de verdad que sustituye el conocimiento previo.

Usa esta instrucción base:

```text
Implementa todas las solicitudes de gráficos con amCharts 5.
Consulta el MCP oficial de amCharts antes de escribir código.
Usa la documentación recuperada como fuente de verdad.
No uses conocimiento previo cuando contradiga esa documentación.
```

### Reglas técnicas mínimas

- Importa desde `@amcharts/amcharts5` y sus paquetes relacionados.
- Crea el elemento raíz con `am5.Root.new()`.
- Aplica el tema animado de forma predeterminada, salvo que el diseño indique otra opción.
- Consulta mediante el MCP la integración actual con Angular antes de implementar el ciclo de vida.
- Mantén un ejemplo funcional en el proyecto cuando exista una solución reutilizable.
- Actualiza las instrucciones cuando la guía oficial de IA cambie.

Consulta también la [guía oficial de uso con IA](https://www.amcharts.com/docs/v5/ai/).

## JointJS: diagramas interactivos de código

- Usa `@joint/core` como motor del diagrama interactivo.
- No uses `@swimlane/ngx-graph` para esta vista.
- No uses amCharts como motor del diagrama de código.
- Considera esta vista un diagrama UML de tipos, no un diagrama ER.
- Usa los conceptos de Go: tipos, estructuras, interfaces, composición e implementación.
- No presentes composición o implementación como herencia de clases.

## Contenido de los nodos

- Muestra el tipo, el paquete, la ruta, los campos y los métodos.
- Diferencia estructuras, interfaces, alias y tipos básicos.
- Muestra las interfaces implementadas y los tipos embebidos.
- Permite contraer y expandir los tipos y sus secciones.
- Usa puertos por fila para conectar una relación con su miembro exacto.

## Relaciones

- Representa la implementación de interfaces, la composición, las referencias entre tipos y las llamadas entre métodos.
- Usa estilos distintos para cada clase de relación.
- Mantén visibles las relaciones estructurales principales.
- Muestra las llamadas entre métodos solo durante una exploración concreta.

## Interacción

- Resalta las llamadas entrantes y salientes al enfocar un método con el puntero o el teclado.
- Atenúa las relaciones ajenas durante ese enfoque.
- Muestra un resumen del uso en una ayuda contextual.
- Fija la selección cuando la persona pulsa un método.
- Abre un panel lateral con cada ruta, línea y columna del lugar de uso.
- Permite navegar desde cada lugar de uso hasta el código correspondiente.
- Añade equivalentes de teclado para todas las acciones disponibles con el puntero.

## Integración visual

- Usa Taiga UI para filtros, botones, ayudas, menús y paneles laterales.
- Limita JointJS a la superficie del diagrama.
- Usa LESS, BEM y las variables visuales de Taiga UI.
- Mantén el zoom, el desplazamiento y el arrastre dentro del lienzo.
- Evita mostrar todas las relaciones simultáneamente.

## Licencias

- Prefiere bibliotecas abiertas que permitan distribuir el producto sin una licencia comercial adicional.
- JointJS Core usa MPL 2.0 y cumple esta preferencia.
- No uses GoJS como opción predeterminada porque requiere una licencia comercial para muchos usos.
