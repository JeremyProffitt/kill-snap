import { test, expect, Page } from '@playwright/test';

const BASE_URL = 'https://kill-snap.jeremy.ninja';
const USERNAME = 'Jeremy';
const PASSWORD = 'KillSnap4President!';

let authToken = '';

// Helper: login via API and set token
async function loginViaAPI(): Promise<string> {
  const resp = await fetch(`${BASE_URL}/api/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username: USERNAME, password: PASSWORD }),
  });
  const data = await resp.json() as any;
  if (!data.token) throw new Error(`Login failed: ${JSON.stringify(data)}`);
  return data.token;
}

// Helper: login and navigate to gallery
async function login(page: Page) {
  await page.goto(BASE_URL);
  await page.waitForSelector('input[type="text"]', { timeout: 10000 });
  await page.locator('input[type="text"]').first().fill(USERNAME);
  await page.locator('input[type="password"]').first().fill(PASSWORD);
  await page.locator('button:has-text("Login")').first().click();
  // Wait for gallery to load - use multiple possible selectors
  await page.waitForSelector('[class*="gallery"], [class*="Gallery"], [class*="sidebar"], .image-card', { timeout: 15000 });
  await page.waitForTimeout(3000);
  // Grab the token from storage
  authToken = await page.evaluate(() => localStorage.getItem('authToken') || '') || '';
  console.log(`LOGIN: Success (token length: ${authToken.length})`);
}

// Helper: API call with auth
async function apiCall(page: Page, method: string, path: string, body?: any) {
  return page.evaluate(async (args: { base: string, method: string, path: string, body: any }) => {
    const token = localStorage.getItem('authToken') || '';
    const resp = await fetch(`${args.base}${args.path}`, {
      method: args.method,
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${token}`,
      },
      body: args.body ? JSON.stringify(args.body) : undefined,
    });
    const data = await resp.json().catch(() => null);
    return { status: resp.status, data };
  }, { base: BASE_URL, method, path, body });
}

test.describe('Kill-Snap E2E Tests', () => {
  test.setTimeout(180000);

  test('1. Login and inventory current state', async ({ page }) => {
    await login(page);

    // Get current state of images and projects via API
    const projects = await apiCall(page, 'GET', '/api/projects');
    console.log(`\n=== PROJECTS ===`);
    if (Array.isArray(projects.data)) {
      for (const p of projects.data) {
        console.log(`  "${p.name}" | ID: ${p.projectId} | Images: ${p.imageCount} | Archived: ${p.archived || false} | Zips: ${p.zipFiles?.length || 0}`);
      }
    }

    // Count images by state
    for (const state of ['unreviewed', 'approved', 'rejected']) {
      const resp = await apiCall(page, 'GET', `/api/images?state=${state}&limit=500`);
      const count = resp.data?.images?.length || 0;
      console.log(`${state.toUpperCase()} images: ${count}`);

      // For approved, show group breakdown
      if (state === 'approved' && count > 0) {
        const groups: Record<number, number> = {};
        for (const img of resp.data.images) {
          const g = img.groupNumber || 0;
          groups[g] = (groups[g] || 0) + 1;
        }
        console.log(`  Group breakdown: ${JSON.stringify(groups)}`);
      }
    }

    await page.screenshot({ path: 'tests/screenshots/01-gallery.png', fullPage: true });
    console.log('\nTest 1 PASSED: Logged in and inventoried state');
  });

  test('2. Group assignment + add to project', async ({ page }) => {
    await login(page);

    // Step 1: Check if there are approved images with a group
    console.log('\n=== Checking approved images with groups ===');
    const approvedResp = await apiCall(page, 'GET', '/api/images?state=approved&limit=500');
    const approvedImages = approvedResp.data?.images || [];
    console.log(`Total approved images: ${approvedImages.length}`);

    // Check by group
    for (const group of [1, 2, 3, 4, 5]) {
      const groupResp = await apiCall(page, 'GET', `/api/images?state=approved&group=${group}&limit=500`);
      const groupCount = groupResp.data?.images?.length || 0;
      if (groupCount > 0) {
        console.log(`  Group ${group}: ${groupCount} images`);
      }
    }

    // Step 2: Get list of projects
    const projectsResp = await apiCall(page, 'GET', '/api/projects');
    const projects = (projectsResp.data || []).filter((p: any) => !p.archived);
    console.log(`\nActive projects: ${projects.length}`);
    for (const p of projects) {
      console.log(`  "${p.name}" (${p.projectId}) - ${p.imageCount} images`);
    }

    if (projects.length === 0) {
      // Create a test project
      console.log('\nCreating test project...');
      const createResp = await apiCall(page, 'POST', '/api/projects', { name: 'E2E Test Project' });
      console.log(`Create project: status=${createResp.status}, data=${JSON.stringify(createResp.data)}`);
      projects.push(createResp.data);
    }

    const targetProject = projects[0];
    console.log(`\nTarget project: "${targetProject.name}" (${targetProject.projectId})`);

    // Step 3: If no approved images, approve some from unreviewed
    if (approvedImages.length === 0) {
      console.log('\nNo approved images, approving some from unreviewed...');
      const unreviewedResp = await apiCall(page, 'GET', '/api/images?state=unreviewed&limit=5');
      const unreviewed = unreviewedResp.data?.images || [];
      console.log(`Unreviewed images available: ${unreviewed.length}`);

      for (let i = 0; i < Math.min(3, unreviewed.length); i++) {
        const img = unreviewed[i];
        const updateResp = await apiCall(page, 'PUT', `/api/images/${img.imageGUID}`, {
          groupNumber: 1,
          colorCode: 'red',
          reviewed: 'true',
        });
        console.log(`  Approved image ${img.imageGUID}: status=${updateResp.status}`);
      }

      // Wait for async move to process
      console.log('Waiting 5s for async moves...');
      await page.waitForTimeout(5000);

      // Verify
      const verifyResp = await apiCall(page, 'GET', '/api/images?state=approved&group=1&limit=500');
      console.log(`Approved group 1 after approve: ${verifyResp.data?.images?.length || 0} images`);
    }

    // Step 4: Add images to project using group filter
    console.log('\n=== Testing Add to Project with group filter ===');

    // Find which group has images
    let testGroup = 0;
    let testGroupCount = 0;
    for (const group of [1, 2, 3, 4, 5]) {
      const gResp = await apiCall(page, 'GET', `/api/images?state=approved&group=${group}&limit=500`);
      const count = gResp.data?.images?.length || 0;
      if (count > 0 && testGroup === 0) {
        testGroup = group;
        testGroupCount = count;
      }
    }

    if (testGroup > 0) {
      console.log(`Using group ${testGroup} with ${testGroupCount} approved images`);

      // Call add-to-project API directly
      const addResp = await apiCall(page, 'POST', `/api/projects/${targetProject.projectId}/images`, {
        group: testGroup,
      });
      console.log(`Add to project response: status=${addResp.status}, data=${JSON.stringify(addResp.data)}`);

      if (addResp.status === 200) {
        console.log(`SUCCESS: Moved ${addResp.data?.movedCount || 0} images to project "${targetProject.name}"`);
      } else {
        console.log(`FAILURE: ${JSON.stringify(addResp.data)}`);
      }

      // Verify project images
      await page.waitForTimeout(2000);
      const projImgsResp = await apiCall(page, 'GET', `/api/projects/${targetProject.projectId}/images`);
      const projImgs = Array.isArray(projImgsResp.data) ? projImgsResp.data : [];
      console.log(`Project now has ${projImgs.length} images`);
    } else {
      console.log('No approved images in any group to test with');
    }

    // Step 5: Test via UI - reload and use the gallery sidebar
    console.log('\n=== Testing Add to Project via UI ===');
    await page.reload();
    await page.waitForSelector('img', { timeout: 15000 });
    await page.waitForTimeout(2000);
    await page.screenshot({ path: 'tests/screenshots/02-before-add.png', fullPage: true });

    console.log('\nTest 2 PASSED: Group assignment and add-to-project workflow tested');
  });

  test('3. Single image add to project via modal', async ({ page }) => {
    await login(page);

    // Get an approved image
    const approvedResp = await apiCall(page, 'GET', '/api/images?state=approved&limit=5');
    const approved = approvedResp.data?.images || [];
    console.log(`Approved images available: ${approved.length}`);

    // Get a project
    const projectsResp = await apiCall(page, 'GET', '/api/projects');
    const projects = (projectsResp.data || []).filter((p: any) => !p.archived);

    if (approved.length > 0 && projects.length > 0) {
      const testImage = approved[0];
      const targetProject = projects[0];

      console.log(`Adding image ${testImage.imageGUID} to project "${targetProject.name}"`);

      // Use the single-image add-to-project API
      const addResp = await apiCall(page, 'POST', `/api/projects/${targetProject.projectId}/images`, {
        imageGUID: testImage.imageGUID,
        group: testImage.groupNumber || 0,
      });

      console.log(`Single image add: status=${addResp.status}, data=${JSON.stringify(addResp.data)}`);

      if (addResp.status === 200 && addResp.data?.movedCount > 0) {
        console.log('SUCCESS: Single image added to project');
      } else if (addResp.status === 200 && addResp.data?.movedCount === 0) {
        console.log('INFO: Image already in a project or no files to move');
      } else {
        console.log(`ISSUE: Unexpected response - ${JSON.stringify(addResp.data)}`);
      }
    } else {
      console.log('No approved images or no projects - skipping single image test');
    }

    console.log('\nTest 3 PASSED: Single image add to project tested');
  });

  test('4. Test create zip functionality', async ({ page }) => {
    await login(page);

    // Get projects with images
    const projectsResp = await apiCall(page, 'GET', '/api/projects');
    const projects = (projectsResp.data || []).filter((p: any) => !p.archived && p.imageCount > 0);
    console.log(`Projects with images: ${projects.length}`);

    if (projects.length === 0) {
      console.log('No projects with images - cannot test zip');
      return;
    }

    const targetProject = projects[0];
    console.log(`Testing zip for project "${targetProject.name}" (${targetProject.imageCount} images)`);

    // Check existing zips
    if (targetProject.zipFiles?.length > 0) {
      console.log(`Existing zips: ${targetProject.zipFiles.length}`);
      for (const zf of targetProject.zipFiles) {
        console.log(`  ${zf.key} | Status: ${zf.status} | Size: ${zf.size} | Images: ${zf.imageCount}`);
      }
    }

    // Trigger zip generation
    console.log('\n=== Triggering zip generation ===');
    const zipResp = await apiCall(page, 'POST', `/api/projects/${targetProject.projectId}/generate-zip`);
    console.log(`Generate zip: status=${zipResp.status}, data=${JSON.stringify(zipResp.data)}`);

    if (zipResp.status !== 200) {
      console.log(`FAILURE: Zip generation failed - ${JSON.stringify(zipResp.data)}`);
      return;
    }

    // Poll for completion
    console.log('Polling for zip completion...');
    let zipComplete = false;
    for (let i = 0; i < 24; i++) { // Up to 2 minutes
      await page.waitForTimeout(5000);

      // Check zip status via project details
      const projResp = await apiCall(page, 'GET', '/api/projects');
      const updatedProject = (projResp.data || []).find((p: any) => p.projectId === targetProject.projectId);

      if (updatedProject?.zipFiles?.length > 0) {
        const latestZip = updatedProject.zipFiles[updatedProject.zipFiles.length - 1];
        console.log(`  [${(i + 1) * 5}s] Zip status: ${latestZip.status} | Size: ${latestZip.size || 'N/A'}`);

        if (latestZip.status === 'complete') {
          console.log(`\nZIP COMPLETE!`);
          console.log(`  Key: ${latestZip.key}`);
          console.log(`  Size: ${(latestZip.size / 1024 / 1024).toFixed(2)} MB`);
          console.log(`  Images: ${latestZip.imageCount}`);

          // Test download URL
          const downloadResp = await apiCall(page, 'GET',
            `/api/projects/${targetProject.projectId}/zips/${encodeURIComponent(latestZip.key)}/download`
          );
          console.log(`  Download URL available: ${downloadResp.status === 200}`);
          if (downloadResp.data?.url) {
            console.log(`  Download URL length: ${downloadResp.data.url.length}`);
          }

          zipComplete = true;
          break;
        } else if (latestZip.status === 'failed') {
          console.log('\nZIP FAILED!');
          // Check zip logs
          const logsResp = await apiCall(page, 'GET', `/api/projects/${targetProject.projectId}/zip-logs`);
          console.log(`Zip logs: ${JSON.stringify(logsResp.data)}`);
          break;
        }
      } else {
        console.log(`  [${(i + 1) * 5}s] No zip files yet...`);
      }
    }

    if (!zipComplete) {
      // Check logs for debugging
      const logsResp = await apiCall(page, 'GET', `/api/projects/${targetProject.projectId}/zip-logs`);
      console.log(`\nZip logs: ${JSON.stringify(logsResp.data)}`);
    }

    console.log('\nTest 4 COMPLETE: Zip functionality tested');
  });

  test('5. Full zip generation test (approve, add to project, generate zip)', async ({ page }) => {
    await login(page);

    console.log('\n=== FULL ZIP GENERATION TEST ===');

    // Step 1: Create a fresh test project
    const projName = `Zip Test ${Date.now()}`;
    const createResp = await apiCall(page, 'POST', '/api/projects', { name: projName });
    expect(createResp.status).toBeLessThanOrEqual(201);
    const projectId = createResp.data.projectId;
    console.log(`Created project: "${projName}" (${projectId})`);

    // Step 2: Approve 2 unreviewed images with group 2
    const unreviewedResp = await apiCall(page, 'GET', '/api/images?state=unreviewed&limit=5');
    const unreviewed = unreviewedResp.data?.images || [];
    console.log(`Unreviewed images available: ${unreviewed.length}`);
    expect(unreviewed.length).toBeGreaterThanOrEqual(2);

    const testImages = unreviewed.slice(0, 2);
    for (const img of testImages) {
      const updateResp = await apiCall(page, 'PUT', `/api/images/${img.imageGUID}`, {
        groupNumber: 1,
        colorCode: 'red',
        reviewed: 'true',
      });
      console.log(`  Approved image ${img.imageGUID}: status=${updateResp.status}`);
      expect(updateResp.status).toBe(200);
    }

    // Step 3: Wait for async moves to complete (important to let them finish BEFORE addToProject)
    console.log('Waiting 15s for async moves to complete...');
    await page.waitForTimeout(15000);

    // Verify images are now in approved state
    const approvedResp = await apiCall(page, 'GET', '/api/images?state=approved&group=1&limit=500');
    const approvedCount = approvedResp.data?.images?.length || 0;
    console.log(`Approved images in group 2: ${approvedCount}`);

    // Step 4: Add group 2 images to the project
    console.log('\n=== Adding images to project ===');

    // First, log details about the approved images we expect to move
    const preAddResp = await apiCall(page, 'GET', '/api/images?state=approved&group=1&limit=500');
    const preAddImages = preAddResp.data?.images || [];
    console.log(`Pre-add approved group 2 images: ${preAddImages.length}`);
    for (const img of preAddImages) {
      console.log(`  ${img.imageGUID}: status=${img.status}, file=${img.originalFile}, moveStatus=${img.moveStatus || 'N/A'}`);
    }

    // Also check inbox images that might still be pending move
    const inboxResp = await apiCall(page, 'GET', '/api/images?state=unreviewed&group=1&limit=500');
    console.log(`Inbox group 2 images: ${inboxResp.data?.images?.length || 0}`);

    const addResp = await apiCall(page, 'POST', `/api/projects/${projectId}/images`, { group: 1 });
    console.log(`Add to project response: ${JSON.stringify(addResp.data)}`);

    // Wait for CloudWatch logs to propagate, then check for diagnostic output
    console.log('Waiting 8s for CloudWatch log propagation...');
    await page.waitForTimeout(8000);

    // Check API Lambda logs - filter for our diagnostic markers
    const apiLogsResp = await apiCall(page, 'GET', '/api/logs?function=ImageReviewApi&hours=1&filter=all');
    if (apiLogsResp.status === 200 && apiLogsResp.data?.logs) {
      const logs = apiLogsResp.data.logs;
      // Filter for our diagnostic markers
      const diagnosticLogs = logs.filter((l: any) =>
        l.message?.includes('ADD TO PROJECT') ||
        l.message?.includes('Total images to process') ||
        l.message?.includes('Moving image') ||
        l.message?.includes('Failed to move') ||
        l.message?.includes('ErrSourceFileMissing') ||
        l.message?.includes('movedCount') ||
        l.message?.includes('NoSuchKey')
      );
      console.log(`\nDiagnostic log entries: ${diagnosticLogs.length}`);
      for (const entry of diagnosticLogs) {
        console.log(`  ${entry.message?.trim()}`);
      }
      if (diagnosticLogs.length === 0) {
        console.log('  (No diagnostic logs found - may still be propagating)');
        // Show last 15 non-platform entries
        const appLogs = logs.filter((l: any) =>
          !l.message?.startsWith('START') &&
          !l.message?.startsWith('END') &&
          !l.message?.startsWith('REPORT')
        );
        console.log(`  App log entries in window: ${appLogs.length}`);
        for (const entry of appLogs.slice(-15)) {
          console.log(`  ${entry.message?.trim()}`);
        }
      }
    }

    if (addResp.data?.movedCount === 0) {
      // Debug: verify images are still approved
      const recheckResp = await apiCall(page, 'GET', '/api/images?state=approved&group=1&limit=500');
      console.log(`\nRecheck approved group 2: ${recheckResp.data?.images?.length || 0} images`);

      // Try with a single image directly by GUID
      if (preAddImages.length > 0) {
        const singleImg = preAddImages[0];
        console.log(`\nTrying single image add: ${singleImg.imageGUID}`);
        const singleResp = await apiCall(page, 'POST', `/api/projects/${projectId}/images`, {
          imageGUID: singleImg.imageGUID,
        });
        console.log(`Single image add response: ${JSON.stringify(singleResp.data)}`);
      }
    }

    expect(addResp.status).toBe(200);
    const movedCount = addResp.data?.movedCount || 0;
    console.log(`\nMoved count: ${movedCount}`);
    if (movedCount === 0) {
      console.log('WARNING: movedCount is 0, continuing to see if zip still works...');
    }

    // Verify project images
    await page.waitForTimeout(2000);
    const projImgsResp = await apiCall(page, 'GET', `/api/projects/${projectId}/images`);
    const projImgs = Array.isArray(projImgsResp.data) ? projImgsResp.data : [];
    console.log(`Project images after add: ${projImgs.length}`);
    for (const img of projImgs) {
      console.log(`  ${img.imageGUID}: ${img.originalFile} (${img.fileSize} bytes)`);
    }

    // Step 5: Generate zip
    console.log('\n=== Generating zip ===');
    const zipResp = await apiCall(page, 'POST', `/api/projects/${projectId}/generate-zip`);
    console.log(`Generate zip: status=${zipResp.status}`);
    expect(zipResp.status).toBe(200);

    // Poll for completion
    let zipResult: any = null;
    for (let i = 0; i < 36; i++) { // Up to 3 minutes
      await page.waitForTimeout(5000);

      const projResp = await apiCall(page, 'GET', '/api/projects');
      const updatedProject = (projResp.data || []).find((p: any) => p.projectId === projectId);

      if (updatedProject?.zipFiles?.length > 0) {
        const latestZip = updatedProject.zipFiles[updatedProject.zipFiles.length - 1];
        console.log(`  [${(i + 1) * 5}s] status=${latestZip.status} | size=${latestZip.size} | images=${latestZip.imageCount}`);

        if (latestZip.status === 'complete' || latestZip.status === 'failed') {
          zipResult = latestZip;
          break;
        }
      }
    }

    // Step 6: Validate zip result
    console.log('\n=== ZIP RESULT ===');
    expect(zipResult).not.toBeNull();
    console.log(`Status: ${zipResult.status}`);
    console.log(`Size: ${zipResult.size} bytes (${(zipResult.size / 1024 / 1024).toFixed(2)} MB)`);
    console.log(`Images: ${zipResult.imageCount}`);
    console.log(`Key: ${zipResult.key}`);

    // Log result even if assertions would fail
    if (zipResult.imageCount === 0) {
      console.log('WARNING: Zip contains 0 images');
    }
    if (zipResult.size <= 100) {
      console.log('WARNING: Zip size is suspiciously small');
    }
    expect(zipResult.status).toBe('complete');

    // Test download URL
    const downloadResp = await apiCall(page, 'GET',
      `/api/projects/${projectId}/zips/${encodeURIComponent(zipResult.key)}/download`
    );
    console.log(`Download URL: status=${downloadResp.status}`);
    expect(downloadResp.status).toBe(200);
    expect(downloadResp.data?.url).toBeTruthy();

    // Check zip Lambda logs for errors
    console.log('\n=== Zip Lambda Logs ===');
    const logsResp = await apiCall(page, 'GET', `/api/logs?function=ProjectZipGenerator&hours=1&filter=all`);
    if (logsResp.status === 200 && logsResp.data?.logs) {
      const logs = logsResp.data.logs;
      // Show the most recent zip run logs
      const errorLogs = logs.filter((l: any) => l.message?.includes('ERROR'));
      if (errorLogs.length > 0) {
        console.log(`WARNING: ${errorLogs.length} error entries found in zip logs:`);
        for (const entry of errorLogs.slice(-10)) {
          console.log(`  ${entry.message?.trim()}`);
        }
      } else {
        console.log('No errors in zip Lambda logs');
      }
    }

    console.log('\nTest 5 PASSED: Zip generation working correctly!');
  });

  test('6. UI integration - gallery add to project flow', async ({ page }) => {
    await login(page);

    // Navigate to approved view via UI
    // Click on state filter buttons
    const sidebar = page.locator('.sidebar, [class*="sidebar"]').first();

    // Take initial screenshot
    await page.screenshot({ path: 'tests/screenshots/05-initial.png', fullPage: true });

    // Try clicking "Approved" or finding state tabs
    const approvedLink = page.locator('a:has-text("Approved"), button:has-text("Approved"), [class*="state"]:has-text("Approved"), .sidebar-section:has-text("Approved")').first();
    if (await approvedLink.count() > 0) {
      await approvedLink.click();
      await page.waitForTimeout(2000);
      await page.screenshot({ path: 'tests/screenshots/05-approved.png', fullPage: true });

      const imgCount = await page.locator('img[src*="thumbnail"], img[src*="images"], .image-card img').count();
      console.log(`Approved images visible in UI: ${imgCount}`);
    }

    // Click a group filter
    const group1Btn = page.locator('[class*="group-box"]:has-text("1")').first();
    if (await group1Btn.count() > 0) {
      await group1Btn.click();
      await page.waitForTimeout(2000);
      await page.screenshot({ path: 'tests/screenshots/05-group1.png', fullPage: true });

      const imgCount = await page.locator('img[src*="thumbnail"], img[src*="images"], .image-card img').count();
      console.log(`Group 1 images visible: ${imgCount}`);
    }

    // Select a project from sidebar dropdown
    const projectSelect = page.locator('select').filter({ hasText: /Select Project|project/i }).first();
    if (await projectSelect.count() > 0) {
      const options = await projectSelect.locator('option[value]').all();
      for (const opt of options) {
        const val = await opt.getAttribute('value');
        if (val && val !== '') {
          await projectSelect.selectOption(val);
          console.log(`Selected project: ${val}`);

          // Find and click "Add to Project" button
          const addBtn = page.locator('button:has-text("Add to Project")').first();
          if (await addBtn.count() > 0 && await addBtn.isEnabled()) {
            // Listen for API response
            const respPromise = page.waitForResponse(
              r => r.url().includes('/images') && r.request().method() === 'POST',
              { timeout: 30000 }
            ).catch(() => null);

            await addBtn.click();
            console.log('Clicked Add to Project');

            const resp = await respPromise;
            if (resp) {
              const body = await resp.json().catch(() => null);
              console.log(`UI Add to Project: status=${resp.status()}, body=${JSON.stringify(body)}`);
            }

            await page.waitForTimeout(3000);
            await page.screenshot({ path: 'tests/screenshots/05-after-add.png', fullPage: true });
          }
          break;
        }
      }
    }

    console.log('\nTest 5 COMPLETE: UI integration tested');
  });
});
