import React from 'react';
import { BreadcrumbItem } from '../types';

interface BreadcrumbProps {
  items: BreadcrumbItem[];
  onNavigate: (path: string) => void;
}

export const Breadcrumb: React.FC<BreadcrumbProps> = ({ items, onNavigate }) => {
  if (items.length === 0) return null;

  return (
    <nav className="flex items-center space-x-1 text-sm overflow-x-auto whitespace-nowrap pb-2">
      {items.map((item, index) => (
        <React.Fragment key={item.path}>
          {index > 0 && (
            <svg className="w-4 h-4 text-gray-500 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
            </svg>
          )}
          <button
            onClick={() => onNavigate(item.path)}
            className={`flex items-center gap-1 px-2 py-1 rounded transition-colors flex-shrink-0 ${
              index === items.length - 1
                ? 'bg-gray-700/50 text-white font-medium'
                : 'text-gray-400 hover:text-white hover:bg-gray-700/30'
            }`}
          >
            {item.isRoot && (
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6" />
              </svg>
            )}
            <span>{item.name}</span>
          </button>
        </React.Fragment>
      ))}
    </nav>
  );
};

export default Breadcrumb;