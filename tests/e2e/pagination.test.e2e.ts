// @watch start
// template/base/paginate.tmpl
// services/context/pagination.go
// @watch end

import {expect} from '@playwright/test';
import {accessibilityCheck} from './shared/accessibility.ts';
import {test} from './utils_e2e.ts';

test('Pagination a11y', async ({page}, workerInfo) => {
  test.skip(['Mobile Safari', 'Mobile Chrome'].includes(workerInfo.project.name), 'Mobile pagination accessibility has issues');
  await page.goto('/explore/repos');

  await expect(page.locator('.pagination')).toBeVisible();
  await accessibilityCheck({page}, ['.pagination'], [], []);
});
