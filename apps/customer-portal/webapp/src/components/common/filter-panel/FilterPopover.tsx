// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Select,
  MenuItem,
  Button,
  Typography,
  Box,
  IconButton,
  FormControl,
  TextField,
  InputLabel,
  Skeleton,
} from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";
import ErrorIndicator from "@components/common/error-indicator/ErrorIndicator";
import type { SelectChangeEvent } from "@wso2/oxygen-ui";
import { useMemo, useState, type JSX, type ChangeEvent, type UIEvent } from "react";

export interface FilterField {
  id: string;
  label: string;
  type: "select" | "text";
  options?: string[] | { value: string; label: string }[];
  placeholder?: string;
  onLoadMore?: () => void;
  hasMore?: boolean;
  isFetchingMore?: boolean;
}

interface FilterPopoverProps<T extends Record<string, unknown>> {
  open: boolean;
  onClose: () => void;
  onSearch: (filters: T) => void;
  initialFilters: T;
  fields: FilterField[];
  title?: string;
  isLoading?: boolean;
  isError?: boolean;
}

const FilterPopover = <T extends Record<string, unknown>>({
  open,
  onClose,
  onSearch,
  initialFilters,
  fields,
  title = "Advanced Search",
  isLoading = false,
  isError = false,
}: FilterPopoverProps<T>): JSX.Element => {
  const initialTempFilters = useMemo(() => initialFilters, [initialFilters]);
  const [tempFilters, setTempFilters] = useState<T>(() => initialTempFilters);

  const handleSelectChange =
    (field: string) => (event: SelectChangeEvent<string>) => {
      setTempFilters((prev) => ({
        ...prev,
        [field]: event.target.value,
      }) as T);
    };

  const handleTextChange =
    (field: string) => (event: ChangeEvent<HTMLInputElement>) => {
      setTempFilters((prev) => ({
        ...prev,
        [field]: event.target.value,
      }) as T);
    };

  const handleReset = () => {
    const resetState = Object.fromEntries(
      fields.map((field) => [field.id, ""]),
    ) as unknown as T;
    setTempFilters(resetState);
  };

  const handleSearchClick = () => {
    onSearch(tempFilters);
    onClose();
  };

  return (
    <Dialog
      key={open ? "open" : "closed"}
      open={open}
      onClose={onClose}
      fullWidth
      maxWidth="sm"
    >
      {/* filter popover title */}
      <DialogTitle
        sx={{
          borderBottom: 1,
          borderColor: "divider",
          pb: 2,
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
        }}
      >
        {/* filter popover title */}
        <Typography variant="h6" component="span">
          {title}
        </Typography>
        {/* filter popover close button */}
        <IconButton onClick={onClose} size="small" aria-label="close">
          <X size={20} />
        </IconButton>
      </DialogTitle>
      {/* filter popover content */}
      <DialogContent
        sx={{ minHeight: 400, display: "flex", flexDirection: "column" }}
      >
        <Box
          sx={{
            display: "flex",
            flexDirection: "column",
            gap: 3,
            pt: 2,
            flexGrow: 1,
            justifyContent: isError ? "center" : "flex-start",
          }}
        >
          {isLoading ? (
            Array.from({ length: fields.length || 3 }).map((_, index) => (
              <Box key={index} sx={{ width: "100%" }}>
                <Skeleton
                  variant="text"
                  width="30%"
                  height={20}
                  sx={{ mb: 1 }}
                />
                <Skeleton variant="rectangular" width="100%" height={40} />
              </Box>
            ))
          ) : isError ? (
            <Box
              sx={{
                display: "flex",
                flexDirection: "column",
                alignItems: "center",
                justifyContent: "center",
                height: "100%",
                py: 4,
                gap: 1,
              }}
            >
              <ErrorIndicator entityName="filters" />
              <Typography variant="body2" color="error">
                Failed to load filter options
              </Typography>
            </Box>
          ) : (
            fields.map((field) =>
              field.type === "select" ? (
                <FormControl fullWidth size="small" key={field.id}>
                  <InputLabel id={`${field.id}-label`}>
                    {field.label}
                  </InputLabel>
                  <Select<string>
                    labelId={`${field.id}-label`}
                    id={field.id}
                    value={tempFilters[field.id] || ""}
                    label={field.label}
                    onChange={handleSelectChange(field.id)}
                    MenuProps={{
                      PaperProps: {
                        onScroll: (e: UIEvent<HTMLElement>) => {
                          if (
                            !field.onLoadMore ||
                            !field.hasMore ||
                            field.isFetchingMore
                          ) {
                            return;
                          }
                          const el = e.currentTarget;
                          const threshold = 24;
                          const isNearBottom =
                            el.scrollHeight - el.scrollTop - el.clientHeight <
                            threshold;
                          if (isNearBottom) field.onLoadMore();
                        },
                      },
                    }}
                  >
                    {/* filter popover select menu item */}
                    <MenuItem value="">
                      <Typography variant="caption" color="text.disabled">
                        {field.placeholder || `Select ${field.label}`}
                      </Typography>
                    </MenuItem>
                    {/* filter popover select menu items */}
                    {field.options?.map((option) => {
                      const value =
                        typeof option === "string" ? option : option.value;
                      const label =
                        typeof option === "string" ? option : option.label;
                      return (
                        <MenuItem key={value} value={value}>
                          <Typography variant="body2">{label}</Typography>
                        </MenuItem>
                      );
                    })}
                  </Select>
                </FormControl>
              ) : (
                <TextField
                  key={field.id}
                  value={tempFilters[field.id] || ""}
                  onChange={handleTextChange(field.id)}
                  placeholder={field.placeholder || `Enter ${field.label}`}
                  size="small"
                  fullWidth
                  label={field.label}
                />
              ),
            )
          )}
        </Box>
      </DialogContent>
      <DialogActions
        sx={{ px: 3, pb: 2, borderTop: 1, borderColor: "divider", pt: 2 }}
      >
        <Button onClick={handleReset} color="inherit">
          Reset
        </Button>
        <Button onClick={handleSearchClick} variant="contained" color="warning">
          Search
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default FilterPopover;
