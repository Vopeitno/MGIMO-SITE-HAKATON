export const selectArrowCSS=`
select.spaced-select:is(.input,.select){
  appearance:none;-webkit-appearance:none;padding-right:44px;
  background-image:url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='16' height='16' viewBox='0 0 24 24' fill='none' stroke='%2377738a' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpath d='m6 9 6 6 6-6'/%3E%3C/svg%3E");
  background-repeat:no-repeat;background-position:right 16px center;background-size:16px;
}
@media(forced-colors:active){select.spaced-select:is(.input,.select){appearance:auto;background-image:none;padding-right:12px}}
`;
