import { And, Then, When } from '@badeball/cypress-cucumber-preprocessor';
import { TableDefinition } from 'cypress-cucumber-preprocessor';

Then(`user sees a table with headings`, (tableHeadings: TableDefinition) => {
  const headings = tableHeadings.raw()[0];
  cy.get('table');

  headings.forEach(heading => {
    cy.get(`th[data-label="${heading}"]`);
  });
});

And(
  'the {string} column on the {string} row has a link ending in {string}',
  (column: string, rowText: string, link: string) => {
    getColWithRowText(rowText, column).within(() => {
      // $= is endswith since console link can change depending on the deployment.
      cy.get(`a[href$="${link}"]`).should('be.visible');
    });
  }
);

And(
  'the {string} column on the {string} row has the text {string}',
  (column: string, rowText: string, text: string) => {
    getColWithRowText(rowText, column).contains(text);
  }
);

And('the {string} column on the {string} row is empty', (column: string, rowText: string, text: string) => {
  getColWithRowText(rowText, column).children().should('be.empty');
});

And('user clicks in {string} column on the {string} text', (column: string, rowText: string) => {
  getColWithRowText(rowText, column).find('a').click();
});

Then('user sees {string} in the table', (service: string) => {
  cy.get('tbody').within(() => {
    if (service === 'nothing') {
      cy.contains('No services found');
    } else if (service === 'something') {
      cy.contains('No services found').should('not.exist');
    } else {
      cy.contains('td', service);
    }
  });
});

And('table length should be {int}', (numRows: number) => {
  cy.get('tbody').within(() => {
    cy.get('tr').should('have.length', numRows);
  });
});

And('table length should exceed {int}', (numRows: number) => {
  cy.get('tbody').within(() => {
    cy.get('tr').should('have.length.greaterThan', numRows);
  });
});

When('user selects filter {string}', (filter: string) => {
  cy.get('select[aria-label="filter_select_type"]').select(filter);
});

And('user filters for name {string}', (name: string) => {
  cy.get('input[aria-label="filter_input_value"]').type(`${name}{enter}`);
});

And('user filters for istio config type {string}', (istioType: string) => {
  cy.get('input[placeholder="Filter by Istio Config Type"]').type(`${istioType}{enter}`);
  cy.get(`li[label="${istioType}"]`).should('be.visible').find('button').click();
});

// checkCol
// This func assumes:
//
// 1. There is only 1 table on the screen.
//
// Be aware of these assumptions when using this func.
export const colExists = (colName: string, exists: boolean) => {
  return cy.get(`th[data-label="${colName}"]`).should(exists ? 'exist' : 'not.exist');
};

// hasAtLeastOneClass will check if the element has that class/classes.
// This func makes a couple assumptions:
//
// 1. The classes expected
export const hasAtLeastOneClass = (expectedClasses: string[]) => {
  return ($el: HTMLElement[]) => {
    const classList = Array.from($el[0].classList);
    return expectedClasses.some((expectedClass: string) => classList.includes(expectedClass));
  };
};

// getColWithRowText will find the column matching the unique row text and column header name.
// This func makes a couple assumptions:
//
// 1. The text to search for is unique in the row.
// 2. There is only 1 table on the screen.
//
// Be aware of these assumptions when using this func.
export const getColWithRowText = (rowSearchText: string, colName: string) => {
  return cy.get('tbody').contains('tr', rowSearchText).find(`td[data-label="${colName}"]`);
};

// getCellsForCol returns every cell matching the table header name or
// the table header index. Example:
//
// | Name | Type | Health |
// | app1 | wkld | Good   |
// | app2 | svc  | Good   |
//
// getCellsForCol('Name') or getCellsForCol(0) would both return
// the cells 'app1' and 'app2'.
export const getCellsForCol = (column: string | Number) => {
  if (typeof column === 'number') {
    return cy.get('td').eq(column);
  }
  return cy.get(`td[data-label="${column}"]`);
};

Then('user sees the {string} table with {int} rows', (tableName: string, numRows: number) => {
  let tableId = '';

  switch (tableName) {
    case 'Istio Config':
      tableId = 'Istio Config List';
      break;
  }

  cy.get(`table[aria-label="${tableId}"]`).within(() => {
    cy.get('tbody').within(() => {
      cy.get('tr').should('have.length', numRows);
    });
  });
});

// Note that we can't count the rows on this case, as empty tables add a row with the message
Then('user sees the {string} table with empty message', (tableName: string) => {
  let tableId = '';

  switch (tableName) {
    case 'Istio Config':
      tableId = 'Istio Config List';
      break;
  }

  cy.get(`table[aria-label="${tableId}"]`).within(() => {
    cy.get('[data-test="istio-config-empty"]');
  });
});

When('user clicks in the {string} table {string} badge {string} name row link', (tableName, badge, name) => {
  let tableId = '';

  switch (tableName) {
    case 'Istio Config':
      tableId = 'Istio Config List';
      break;
  }

  cy.get(`table[aria-label="${tableId}"]`).within(() => {
    cy.contains('div', badge).siblings().first().click();
  });
});

// ensureObjectsInTable name can represent apps, istio config, objects, services etc.
export const ensureObjectsInTable = (...names: string[]) => {
  cy.get('tbody').within(() => {
    cy.get('tr').should('have.length.at.least', names.length);

    names.forEach(name => {
      cy.get('tr').contains(name);
    });
  });
};

export const checkHealthIndicatorInTable = (
  targetNamespace: string,
  targetType: string | null,
  targetRowItemName: string,
  healthStatus: string
) => {
  const selector = targetType
    ? `${targetNamespace}_${targetType}_${targetRowItemName}`
    : `${targetNamespace}_${targetRowItemName}`;

  cy.get(`tr[data-test=VirtualItem_Ns${selector}]`).find('span').filter(`.icon-${healthStatus}`).should('exist');
};

export const checkHealthStatusInTable = (
  targetNamespace: string,
  targetType: string | null,
  targetRowItemName: string,
  healthStatus: string
) => {
  const selector = targetType
    ? `${targetNamespace}_${targetType}_${targetRowItemName}`
    : `${targetNamespace}_${targetRowItemName}`;

  cy.get(`[data-test=VirtualItem_Ns${selector}] td:first-child span[class=pf-v5-c-icon__content]`).trigger(
    'mouseenter'
  );

  cy.get(`[aria-label='Health indicator'] strong`).should('contain.text', healthStatus);
};

And('an entry for {string} cluster should be in the table', (cluster: string) => {
  cy.get('tbody').within(() => {
    cy.get('tr > td:nth-child(4)').contains(cluster).should('have.length.above', 0);
  });
});
